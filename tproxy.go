package main

import (
	"net"
	"strconv"
	"io"
	"github.com/op/go-logging"
	"os"
	"google.golang.org/api/drive/v3"
	"./climanage"
	//"./toolkits"
	"fmt"
	"bytes"
	"time"
	"io/ioutil"
	"flag"
	"sync"
	"compress/zlib"
)

var log = logging.MustGetLogger("goserver")
var format = logging.MustStringFormatter(
	//`%{time:15:04:05.000} %{shortfunc} > %{level:s} %{message}`,
	`%{time:15:04:05.000} > %{level:s} %{message}`,
)
var lock sync.Mutex

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
	serverId := *cId
	configFile := *cFile
	log.Debug("login from", configFile)
	srv, err := climanage.LoginDriveFromFile(configFile)
	if err != nil{
		panic(err)
	}
	log.Info("login OK")
	log.Debug("server id:", serverId)

	l, err := net.Listen("tcp", ":1085")
	if err != nil {
		log.Panic(err)
	}

	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}

		go handleClientRequest(srv, serverId, client)
		//<- singleCh
		//lock.Unlock()
		log.Debug("next")
	}
}

func handleClientRequest(srv *drive.Service, serverId string, client net.Conn) {
	if client == nil {
		return
	}
	defer client.Close()

	var b [1024]byte
	n, err := client.Read(b[:])
	if err != nil {
		log.Debug(err)
		return
	}

	// 只处理Socks5协议
	// 客户端回应：Socks服务端不需要验证方式
	if b[0] == 0x05 {
		client.Write([]byte{0x05, 0x00})
		n, err = client.Read(b[:])
		var host, port string

		//switch b[3] {
		//case 0x01: //IP V4
		//	host = net.IPv4(b[4], b[5], b[6], b[7]).String()
		//case 0x03: //域名
		//	host = string(b[5 : n-2]) //b[4]表示域名的长度
		//case 0x04: //IP V6
		//	host = net.IP{b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15], b[16], b[17], b[18], b[19]}.String()
		//}

		if (b[3] == 0x01) && (b[4] == 172) {
		//if b[3] == 0x01 {
			host = net.IPv4(b[4], b[5], b[6], b[7]).String()
		} else {
			log.Debug("not allowed IP")
			client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
			return
		}
		port = strconv.Itoa(int(b[n-2])<<8 | int(b[n-1]))

		remoteAddr := fmt.Sprintf("%s:%s", host, port)
		log.Debug("proxy", remoteAddr)

		// 提交请求到远程
		newFile := &drive.File{
			Name: serverId + ".socket",
			MimeType: "application/octet-stream",
			AppProperties: map[string]string{
				"typ":	"proxy",
			},
		}

		// 加密数据
		//sendData := toolkits.AesEncrypt([]byte(remoteAddr), toolkits.MessageKey)
		sendMedia := bytes.NewBuffer([]byte(remoteAddr))

		// 创建请求文件
		nf, err := srv.Files.Create(newFile).Media(sendMedia).Do()
		if err != nil {
			log.Debug("create proxy file error", err.Error())
			client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
			return
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
				return
			}
			//log.Debug("check response")
			if media.Size == 0 {
				outputId = media.Description
				break
			}
		}
		log.Debug("connect", inputId, outputId)

		// 响应客户端连接成功
		client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

		// 进行数据转发
		closeCh := make(chan error)
		//abortCh := make(chan int)
		go GetInputData(srv, inputId, client, closeCh)
		go GetOutputData(srv, outputId, client, closeCh)

		// Wait
		<- closeCh
		log.Debug("proxy close")
		srv.Files.Delete(inputId).Do()
		srv.Files.Delete(outputId).Do()
		//abortCh <- -1
	}
}

func GetInputData(srv *drive.Service, fileId string, src io.Reader, errCh chan error) {
	// 循环等待命令数据
	buf := make([]byte, 10240)
	for {
		// 等待数据
		//log.Debug("begin read")
		n, err := src.Read(buf)
		if err != nil {
			errCh <- err
			errCh <- err
			//log.Debug("client close")
			break
		}
		//log.Debug("read", n, "byte(s)")
		if n == 0 {
			errCh <- err
			errCh <- err
			//log.Debug("client close")
			break
		}
		//tempMedia := bytes.NewBuffer(nil)
		//tempMedia.Write(buf[:n])
		//sendData := toolkits.AesEncrypt(tempMedia.Bytes(), toolkits.MessageKey)
		//sendMedia := bytes.NewBuffer(sendData)
		sendMedia := bytes.NewBuffer(buf[:n])

		// 等待上一次数据被处理
		for {
			media, err := srv.Files.Get(fileId).Fields("size, name, appProperties").Do()
			if err != nil {
				errCh <- err
				return
			}
			if media.Size == 0 {
				break
			}
			time.Sleep(time.Duration(1) * time.Second)
		}

		// 提交数据
		newFile := &drive.File{
			AppProperties: map[string]string{
				"typ":	"data",
			},
		}
		srv.Files.Update(fileId, newFile).Media(sendMedia).Do()
		log.Debug("send", n, "byte(s)")
	}
}

func GetOutputData(srv *drive.Service, fileId string, dst io.Writer, abortCh chan error) {
	// 循环等待命令数据
	for {
		//log.Debug("select")
		timeout := time.NewTicker(time.Second)
		select {
		case <-timeout.C:
			//log.Debug("timeout")
		case <- abortCh:
			//log.Debug("abort")
			return
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
		//recvData := toolkits.AesDecrypt(content, toolkits.MessageKey)
		var out bytes.Buffer
		recvMedia := bytes.NewBuffer(content)
		r, _ := zlib.NewReader(recvMedia)
		io.Copy(&out, r)

		// send to client
		dst.Write(out.Bytes())

		// clear data
		messageMedia := bytes.NewBuffer(nil)
		srv.Files.Update(fileId, nil).Media(messageMedia).Do()
	}

}
