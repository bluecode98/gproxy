package main

import (
	"github.com/op/go-logging"
	"os"
	"google.golang.org/api/drive/v3"
	"./climanage"
	"./toolkits"
	"bytes"
	"time"
	"io/ioutil"
	"flag"
	"net/http"
	"fmt"
	"encoding/json"
)

var log = logging.MustGetLogger("goserver")
var format = logging.MustStringFormatter(
	//`%{time:15:04:05.000} %{shortfunc} > %{level:s} %{message}`,
	`%{time:15:04:05.000} > %{level:s} %{message}`,
)
var gSrv *drive.Service
var	gServerId string



func main() {
	// get param
	cId := flag.String("id", "", "server id")
	cFile := flag.String("c", "", "config file name")
	flag.Parse()

	if (*cId == "") || (*cFile == "") {
		flag.PrintDefaults()
		os.Exit(0)
	}

	// init log config
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format)
	logging.SetBackend(backend2Formatter)

	// 登录Google
	var err error

	gServerId = *cId
	configFile := *cFile
	log.Debug("login from", configFile)
	gSrv, err = climanage.LoginDriveFromFile(configFile)
	if err != nil{
		panic(err)
	}
	log.Info("login OK")
	log.Debug("server id:", gServerId)

	//l, err := net.Listen("tcp", ":1085")
	//if err != nil {
	//	log.Panic(err)
	//}
	httpMux := http.NewServeMux()
	//handle := http.HandlerFunc(msgHandler)
	//httpMux.Handle("/", handle)

	//noHandle := http.NotFoundHandler()
	//httpMux.Handle("*", handle)
	httpMux.HandleFunc("/", NotFoundHandler )


	log.Debug("listening...")
	http.ListenAndServe(":8080", httpMux)


	//handleClientHtmlRequest(srv, serverId)

	//for {
	//	client, err := l.Accept()
	//	if err != nil {
	//		log.Panic(err)
	//	}
	//
	//	go handleClientRequest(srv, serverId, client)
	//	//<- singleCh
	//	//lock.Unlock()
	//	log.Debug("next")
	//}
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	newReq := map[string]string{
		"method": r.Method,
		"path": r.RequestURI,
		"cookie": r.Header.Get("Cookie"),
		"form": r.Form.Encode(),
	}
	d, _ := json.Marshal(newReq)
	log.Debug("proxy", string(d))
	//path := newReq["path"]
	//fmt.Fprint(w, path)
	//fmt.Fprint(w, "not find")
	recvData := handleClientHtmlRequest(gSrv, gServerId, d)
	fmt.Fprint(w, recvData)
}

//func msgHandler(w http.ResponseWriter, r *http.Request) {
//	recvData := handleClientHtmlRequest(gSrv, gServerId)
//	fmt.Fprint(w, recvData)
//}

func handleClientHtmlRequest(srv *drive.Service, serverId string, data []byte) string {
	// 提交请求到远程
	newFile := &drive.File{
		Name: serverId + ".hpp",
		MimeType: "application/octet-stream",
		AppProperties: map[string]string{
			"typ":	"proxy",
		},
	}

	// 加密数据
	sendData := toolkits.AesEncrypt(data, toolkits.MessageKey)
	messageMedia := bytes.NewBuffer(sendData)

	// 创建请求文件
	nf, err := srv.Files.Create(newFile).Media(messageMedia).Do()
	if err != nil {
		log.Debug("create proxy file error", err.Error())
		return err.Error()
	}

	// 文件ID
	inputId := nf.Id
	outputId := ""

	// wait for response
	for {
		timeout := time.NewTicker(time.Second)
		select {
		case <-timeout.C:
			//case <- abortCh:
			//	return
		}

		media, err := srv.Files.Get(inputId).Fields("id, size, description").Do()
		if err != nil {
			log.Debug("get error")
			return err.Error()
		}
		//log.Debug("check response")
		if media.Size == 0 {
			outputId = media.Description
			break
		}
	}
	log.Debug("connect", inputId, outputId)

	// 进行数据转发
	recvData := GetOutputHtml(srv, outputId)

	// Wait
	log.Debug("proxy close")
	srv.Files.Delete(inputId).Do()
	srv.Files.Delete(outputId).Do()
	//abortCh <- -1

	return string(recvData)

}

func GetOutputHtml(srv *drive.Service, fileId string) []byte {
	// 循环等待命令数据
	for {
		//log.Debug("select")
		timeout := time.NewTicker(time.Second)
		select {
		case <-timeout.C:
			log.Debug("wait...")
		//case <- abortCh:
			//log.Debug("abort")
			//return
		}
		// 等待数据
		//log.Debug("get file")
		media, err:= srv.Files.Get(fileId).Fields("size, name, appProperties").Do()
		if err != nil {
			log.Debug("get error", err.Error())
			//errCh <- err
			break
		}
		if media.Size == 0 {
			//time.Sleep(time.Duration(1)*time.Second)
			continue
		}

		// read data from gdrive
		log.Debug("recv", media.Size, "byte(s)")
		response, err := srv.Files.Get(fileId).Download()
		if err != nil {
			log.Debug("Download error:", err)
			break
		}
		content, _ := ioutil.ReadAll(response.Body)
		response.Body.Close()
		recvData := toolkits.AesDecrypt(content, toolkits.MessageKey)

		// clear data
		//messageMedia := bytes.NewBuffer(nil)
		//srv.Files.Update(fileId, nil).Media(messageMedia).Do()

		// send to client
		return recvData
	}

	return nil
}
