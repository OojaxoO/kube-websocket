package main

import (
	"time"
	"io"
	"fmt"
	"os"
	"strings"
	"bytes"

	"encoding/json"
	"golang.org/x/net/websocket"
	v1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	metricsV1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type Kubernetes struct {
	ConfigPath string `json:"config_path"`
}

type ResizeTerminal struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type Top struct {
	Cpu  int64  `json:"cpu"`
	Mem  int64  `json:"mem"`
}

type PodTop struct {
	Name string `json:"name"`
	Top
}

type DeployTop struct {
	Pod  []PodTop `json:"pod"`
	Top
}

type ProjectTop struct {
	Deploy map[string]DeployTop `json:"deploy"`
	Top
}

type DeployStatus struct {
	// Name 			string `json:"name"`
	Image           []string  `json:"image"`
	Replicas		int32	`json:"replicas"`
	ReadyReplicas	int32	`json:"readyReplicas"`
}

func (k Kubernetes) steamLog(name, namespace string) (io.ReadCloser, error) {
	defer os.RemoveAll(k.ConfigPath)
	config, err := clientcmd.BuildConfigFromFlags("", k.ConfigPath)
	if err != nil {
		return *new(io.ReadCloser), err
	}
	clientset, _ := kubernetes.NewForConfig(config)
	req := clientset.CoreV1().Pods(namespace).GetLogs(name,
		&v1.PodLogOptions{Follow: true, TailLines: func(i int64) *int64 {return &i}(500)})
	reader, err := req.Stream()
	if err != nil {
		return *new(io.ReadCloser), err
	}
	return reader, nil
}

func (k Kubernetes) execBash(name, namespace string, ws *websocket.Conn) error {
	defer os.RemoveAll(k.ConfigPath)
	config, err := clientcmd.BuildConfigFromFlags("", k.ConfigPath)
	if err != nil {
		return err
	}
	clientset, _ := kubernetes.NewForConfig(config)
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(
			&v1.PodExecOptions{
				Command: []string{"/bin/bash"},
				Stdin:   true,
				Stdout:  true,
				Stderr:  true,
				TTY:     true,
			}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	read, write := io.Pipe()
	que := Que{Size: make(chan remotecommand.TerminalSize, 10)}
	go func() {
		buf := make([]byte, 256)
		for {
			var arg ResizeTerminal
			l, err := ws.Read(buf)
			if err != nil{
				_ = write.Close()
				return
			}
			err = json.Unmarshal(buf[:l], &arg)
			if err == nil {
				fmt.Print(arg)
				size := remotecommand.TerminalSize{Width: uint16(arg.Cols), Height: uint16(arg.Rows)}
				que.Size <- size
				continue
			}
			_, _ = write.Write(buf[:l])
		}
	}()
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:             read,
		Stdout:            ws,
		Stderr:            ws,
		Tty:               true,
		TerminalSizeQueue: que,
	})
	if err != nil {
		return err
	}
	return nil
}

func (k Kubernetes) podStatus(name, namespace string, ws *websocket.Conn) error {
	config, err := clientcmd.BuildConfigFromFlags("", k.ConfigPath)
	if err != nil {
		fmt.Print(err)
		return err
	}
	clientset, _ := kubernetes.NewForConfig(config)
	podList, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "app="+name})
	if err != nil {
		fmt.Print(err)
		return err
	}
	jsonData, _ := json.Marshal(podList)
	_, _ = ws.Write(jsonData)

	w, _ := clientset.CoreV1().Pods(namespace).Watch(metav1.ListOptions{LabelSelector: "app="+name})
	go func() {
		buf := make([]byte, 256)
		for {
			_, err := ws.Read(buf)
			if err != nil {
				w.Stop()
				_, _ = ws.Write([]byte(err.Error()))
				return
			}
		}
	}()
	for event := range w.ResultChan() {
		_, ok := event.Object.(*v1.Pod)
		if !ok {
			fmt.Print("unexpected type")
		}
		podList, _ := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "app="+name})
		jsonData, _ := json.Marshal(podList)
		_, _ = ws.Write(jsonData)
	}
	return nil
}

func (k Kubernetes) deploymentStatus(namespace string, ws *websocket.Conn) error {
	defer os.RemoveAll(k.ConfigPath)
	abort := make(chan struct{}) 
	go watchWsAbort(abort, ws) 
	config, err := clientcmd.BuildConfigFromFlags("", k.ConfigPath)
	if err != nil {
		return err
	}
	clientset, _ := kubernetes.NewForConfig(config)
	// deploymentsClient := clientset.AppsV1().Deployments(namespace)
	deploymentsClient := clientset.AppsV1().StatefulSets(namespace)
    // opts := metav1.ListOptions{LabelSelector: "project="+name}
    opts := metav1.ListOptions{}
	watch, err := deploymentsClient.Watch(opts)
	if err != nil {
		return err
	}
	defer watch.Stop()
	for {
		select {
		case event, ok := <- watch.ResultChan(): 
			if !ok {
				return nil // watch关闭时候	
			}
			eventType := event.Type 
			d, ok := event.Object.(*appsv1.StatefulSet)
			if !ok {
				fmt.Print("unexpected type")
				return nil
			}
			deployStatusMap := make(map[string]DeployStatus)
			eventData := make(map[string]interface{})
			image := []string{}
			for _, c := range d.Spec.Template.Spec.Containers {
				image = append(image, c.Image)
			}
			status := DeployStatus{
				Replicas: d.Status.Replicas,
				ReadyReplicas: d.Status.ReadyReplicas,
				Image: image,
			}
			deployStatusMap[d.Name] = status
			eventData["type"] = eventType
			eventData["data"] = deployStatusMap
			jsonData, err := json.Marshal(eventData)
			if err != nil {
				fmt.Println(err)
				break //跳出select 进入下次select	
			}
			go func() {
				ws.Write(jsonData)
			}()
        case <- abort:
			return nil
		}
	}
	return nil
}

func watchTop (opts metav1.ListOptions, podMetricses metricsV1beta1.PodMetricsInterface, ws *websocket.Conn) ([]byte, error) {
	top, err := podMetricses.List(opts)
	if err != nil {
		fmt.Print(err)
		return nil, err
	}
	deployTop := make(map[string]DeployTop)
	var projectTotalCpu, projectTotalMem int64
	for _, podMetrics := range top.Items {
		podName := podMetrics.Name
		podNameSlice := strings.Split(podName, "-")
		deployName := strings.Join(podNameSlice[:len(podNameSlice)-2], "-")
		var cpu, mem int64
		for _, container := range podMetrics.Containers {
			usage := container.Usage
			nowCpu := usage.Cpu().MilliValue()
			nowMem, _ := usage.Memory().AsInt64()
			cpu += nowCpu 
			mem += nowMem 
		}
		pod := PodTop{
			Name: podName,
			Top: Top{
				Cpu: cpu,
				Mem: mem,
			},
		}
		top := deployTop[deployName]
		top.Pod = append(top.Pod, pod)
		top.Cpu += cpu 
		top.Mem += mem
		projectTotalCpu += cpu
		projectTotalMem += mem 
		deployTop[deployName] = top
	}
	projectTop := ProjectTop{
		Top: Top{
			Cpu: projectTotalCpu,
			Mem: projectTotalMem,
		},
		Deploy: deployTop,
	}
	jsonData, _ := json.Marshal(projectTop)
	return jsonData, nil
}

func watchWsAbort (abort chan<- struct{}, ws *websocket.Conn) {
	for {
		msg := ""
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			break	
		}
	}
	abort <- struct{} {} 
}

func (k Kubernetes) deploymentTop(namespace string, ws *websocket.Conn) error {
	defer os.RemoveAll(k.ConfigPath)
	abort := make(chan struct{}) 
	go watchWsAbort(abort, ws) 
	config, err := clientcmd.BuildConfigFromFlags("", k.ConfigPath)
	if err != nil {
		fmt.Print(err)
		return err
	}
	mc, _ := metrics.NewForConfig(config)
	v1beta1Client := mc.MetricsV1beta1()
	podMetricses := v1beta1Client.PodMetricses(namespace)
	opts := metav1.ListOptions{}
	tick := time.Tick(3 * time.Second)
	data, err := watchTop(opts, podMetricses, ws)
	var preData []byte 
	if err != nil {
		return err 
	} else {
		go func () {
			ws.Write(data)
		}()
		preData = data
	}
	for {
		select {
		case <- tick: 
			data, err = watchTop(opts, podMetricses, ws)
			if err != nil {
				return err 
			}
			if (!bytes.Equal(data, preData)) {
				go func () {
					ws.Write(data)
				}()
				preData = data
			}
		case <- abort:
			return nil
		}
	}
	return nil
}

type Que struct {
	Size chan remotecommand.TerminalSize
}

func (q Que) Next() *remotecommand.TerminalSize {
	var x remotecommand.TerminalSize
	x = <-q.Size
	fmt.Print(x)
	return &x
}
