package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
)

type Window struct {
	width  int
	height int
}

func sshClient(username, password, addr string, windowSize <-chan Window) error {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	// Connect to ssh server
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		log.Print("unable to connect: ", err)
		return err
	}

	// Create a session
	session, err := conn.NewSession()
	defer conn.Close()
	defer session.Close()
	if err != nil {
		log.Print("unable to create session: ", err)
		return err
	}

	modes := ssh.TerminalModes{
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	// Request pseudo terminal
	//fmt.Printf("terminal %d %d\n", windowSize.width, windowSize.height)
	if err := session.RequestPty("xterm", 42, 169, modes); err != nil {
		log.Print("request for pseudo terminal failed: ", err)
		return err
	}
	//监测窗口改变
	go func() {
		for {
			select {
			case w := <-windowSize:
				fmt.Print(w)
				_ = session.WindowChange(w.height, w.width)
			}
		}
	}()
	sshIn, _ := session.StdinPipe()
	sshOut, _ := session.StdoutPipe()
	sshErr, _ := session.StderrPipe()
	go io.Copy(sshIn, os.Stdin)
	go io.Copy(os.Stdout, sshOut)
	go io.Copy(os.Stderr, sshErr)
	session.Shell()
	session.Wait()
	return nil
}

func sshServer() {
	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{
		// Remove to disable password auth.
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in
			// a production setting.
			if c.User() == "wuchenzhi" && string(pass) == "wczyyr9293" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}
	privateBytes, err := ioutil.ReadFile("/Users/coolcat/.ssh/id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config.AddHostKey(private)
	// Once a ServerConfig has been configured, connections can be
	// accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:2022")
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}
	nConn, err := listener.Accept()
	if err != nil {
		log.Fatal("failed to accept incoming connection: ", err)
	}

	// Before use, a handshake must be performed on the incoming
	// net.Conn.
	_, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Fatal("failed to handshake: ", err)
	}
	//log.Printf("logged in with key %s", conn.Permissions.Extensions["pubkey-fp"])

	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of a shell, the type is
		// "session" and ServerShell may be used to present a simple
		// terminal interface.
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Fatalf("Could not accept channel: %v", err)
		}

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "shell" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				switch req.Type {
				case "shell":
					req.Reply(req.Type == "shell", nil)
				case "pty-req":
					io.Copy(channel, channel)
				case "window-change":

				}
				req.Reply(req.Type == "shell", nil)
			}
		}(requests)

		//term := terminal.NewTerminal(channel, "> ")
		//
		//go func() {
		//	defer channel.Close()
		//	for {
		//		line, err := term.ReadLine()
		//		if err != nil {
		//			break
		//		}
		//		fmt.Println(line)
		//	}
		//}()
	}
}

// func main() {
// 	//sshServer()
// 	windowSize := make(chan Window)
// 	err := sshClient("root", "wczyyr9293", "10.80.30.103:22", windowSize)
// 	if err != nil {
// 		fmt.Print(err)
// 	}
// 	return
// }

