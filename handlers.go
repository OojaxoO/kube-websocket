package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
)

//func terminal(c echo.Context) error {
//	log.Print("in websocket")
//	ip := c.Param("ip")
//	windowSize := make(chan Window)
//	session, err := sshClient("root", "Llsroot@docker!", ip+":22", windowSize)
//	if err != nil {
//		log.Print(err)
//	}
//	defer session.Close()
//	targetStdout, _ := session.StdoutPipe()
//	targetStdin, _ := session.StdinPipe()
//	targetStderr, _ := session.StderrPipe()
//	_ = session.Shell()
//	log.Print("连接远端成功")
//	websocket.Handler(func(ws *websocket.Conn) {
//		defer ws.Close()
//		go io.Copy(ws, targetStdout)
//		go io.Copy(targetStdin, ws)
//		go io.Copy(ws, targetStderr)
//		_ = session.Wait()
//	}).ServeHTTP(c.Response(), c.Request())
//	return nil
//}

func execBash(c echo.Context) error {
	cluster, _ := strconv.Atoi(c.Param("cluster"))
	namespace := c.Param("namespace")
	name := c.Param("name")
	k := Kubernetes{getKubeConfigFile(cluster)}
	log.Print("连接远端成功")
	websocket.Handler(func(ws *websocket.Conn) {
		//设置websocket payload为2 解决Could not decode a text frame as UTF-8
		ws.PayloadType = websocket.BinaryFrame
		defer ws.Close()
		err := k.execBash(name, namespace, ws)
		if err != nil {
			fmt.Print(err)
		}
		_, _ = ws.Write([]byte("\r\n连接已断开\r\n"))
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func containerLog(c echo.Context) error {
	cluster, _ := strconv.Atoi(c.Param("cluster"))
	namespace := c.Param("namespace")
	name := c.Param("name")
	k := Kubernetes{getKubeConfigFile(cluster)}
	reader, err := k.steamLog(name, namespace)
	if err != nil {
		return err
	}
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		defer reader.Close()
		//设置websocket payload为2 解决Could not decode a text frame as UTF-8
		//ws.PayloadType = websocket.BinaryFrame
		go io.Copy(ws, reader)
		buf := make([]byte, 256)
		for {
			_, err := ws.Read(buf)
			if err != nil {
				_, _ = ws.Write([]byte(err.Error()))
				break
			}
			fmt.Print(buf)
			_, _ = ws.Write(buf)
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func terminalLog(c echo.Context) error {
	cmd := exec.Command("/bin/bash", "-c", "scriptreplay /data/guacamole-text/ops-ubuntu/chenyong--20190711--184604.timing /data/guacamole-text/ops-ubuntu/chenyong--20190711--184604")
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	cmd.Start()
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		//设置websocket payload为2 解决Could not decode a text frame as UTF-8
		ws.PayloadType = websocket.BinaryFrame
		fd := int(os.Stdin.Fd())
		windowSize, _ := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
		fmt.Print(windowSize.Col, windowSize.Row)
		go io.Copy(ws, stdout)
		go io.Copy(ws, stderr)
		go func() {
			defer cmd.Process.Kill()
			buf := make([]byte, 256)
			for {
				_, err := ws.Read(buf)
				if err != nil {
					fmt.Print(err)
					_, _ = ws.Write([]byte(err.Error()))
					break
				}
			}
		}()
		cmd.Wait()
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func servicePod(c echo.Context) error {
	k := Kubernetes{config.Load().(string)}
	namespace := c.Param("namespace")
	name := c.Param("name")
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		_ = k.podStatus(name, namespace, ws)
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func deploymentStatus(c echo.Context) error {
	cluster, _ := strconv.Atoi(c.Param("cluster"))
	namespace := c.Param("namespace")
	// name := c.Param("name")
	k := Kubernetes{getKubeConfigFile(cluster)}
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		_ = k.deploymentStatus(namespace, ws)
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func deploymentTop(c echo.Context) error {
	cluster, _ := strconv.Atoi(c.Param("cluster"))
	namespace := c.Param("namespace")
	// name := c.Param("name")
	k := Kubernetes{getKubeConfigFile(cluster)}
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		_ = k.deploymentTop(namespace, ws)
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}