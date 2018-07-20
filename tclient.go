package main

import (
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"net"
	"crypto/md5"
	"encoding/hex"
)

var addr = flag.String("addr", "localhost:12345", "http service address")

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
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	var dialer *websocket.Dialer

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	// send login message
	clientID := getClientId()
	loginWriter(conn, clientID)

	go timeWriter(conn)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			return
		}

		fmt.Printf("received: %s\n", message)
	}
}

func loginWriter(conn *websocket.Conn, cliendID string)  {
	conn.WriteMessage(websocket.TextMessage, []byte(cliendID))
}

func timeWriter(conn *websocket.Conn) {
	for {
		time.Sleep(time.Second * 2)
		conn.WriteMessage(websocket.TextMessage, []byte(time.Now().Format("2006-01-02 15:04:05")))
	}
}