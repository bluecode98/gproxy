package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"encoding/json"
	"github.com/op/go-logging"
	"os"
	"encoding/csv"
	"time"
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

var connectString = "ws://192.168.4.100:8285"
var groupID = "test"
var log = logging.MustGetLogger("goserver")
var format = logging.MustStringFormatter(
	`%{time:15:04:05.000} > %{message}`,
)

func main() {
	var clientUID string

	if len(connectString)==0 {
		os.Exit(0)
	}
	// init log config
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format)
	logging.SetBackend(backend2Formatter)

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
		log.Debug("login ok")
	} else {
		log.Debug("login error")
		os.Exit(0)
	}

	// 查询所有在线ID
	message = &wsMessage{
		Type:	104,
		Sender:	clientUID,
		Param1: groupID,
	}

	sendData, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, sendData)
	_, messageData, err = conn.ReadMessage()
	if err != nil {
		log.Debug("read:", err)
		return
	}

	// 等待用户返回
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
			//log.Debug(message.Type)
			//log.Debug(message.Param1)
			if message.Type == 105 {
				// 保存主机信息
				log.Debug("save systeminfo", message.Param1)
				infoFilename := fmt.Sprintf(".\\data\\%s\\systeminfo.csv", message.Param1)
				file, err := os.OpenFile(infoFilename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
				if err != nil {
					break
				}
				file.Write([]byte(message.Param2))
				file.Close()
			}
		}
	}()

	message = &wsMessage{}
	err = json.Unmarshal([]byte(messageData), message)
	if err != nil {
		log.Debug("json error", err.Error())
	}
	clientList := make(map[string]interface{})
	json.Unmarshal([]byte(message.Param1), &clientList)
	if len(clientList) == 0 {
		log.Debug("not find live client.")
	} else {
		log.Debug("live client")
		for k, v := range clientList {
			targetUID := k
			serverId := v.(string)
			serverInfo, _ := checkServerInfo(serverId)
			if len(serverInfo)==0 {
				// 查询主机信息
				message = &wsMessage{
					Type:	105,
					Sender:	clientUID,
					Target: targetUID,
				}

				sendData, _ := json.Marshal(message)
				conn.WriteMessage(websocket.TextMessage, sendData)
			} else {
				infoDetail := fmt.Sprintf("%s %s(%s)", serverInfo[0], serverInfo[1], serverInfo[13])
				log.Info(serverId, infoDetail)
			}
		}

		time.Sleep(time.Duration(6)*time.Second)
	}
}

func checkServerInfo(serverId string) ([]string, error) {
	infoFilename := fmt.Sprintf(".\\data\\%s\\systeminfo.csv", serverId)
	// 判断文件是否存在
	_, err := os.Stat(infoFilename)
	if err != nil {
		os.MkdirAll(".\\data\\" + serverId, os.ModePerm)

		return nil, nil
	} else {
		infoFile, err := os.Open(infoFilename)
		defer infoFile.Close()
		if err != nil {
			return nil, err
		}
		reader := csv.NewReader(infoFile)
		line, _ := reader.ReadAll()
		return line[1], nil
	}
}
