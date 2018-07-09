package climanage


import (
	"io"
	"google.golang.org/api/drive/v3"
	"bytes"
	"../toolkits"
	"time"
	"io/ioutil"
)

const MessageKey = "123456781234567812345678"

func UpdateMessage(srv *drive.Service, fileId string, recvId string, sendId string,
	cmdType string, messageMedia io.Reader)(*drive.File, error){
	// update shell result
	newFile := &drive.File{
		//Description: time.Now().Format("2006-01-02 15:04:05"),
		AppProperties: map[string]string{
			"los":	sendId,
			"owe":	recvId,
			"typ":	cmdType,
		},
	}

	nf, err := srv.Files.Update(fileId, newFile).Media(messageMedia).Do()
	return nf, err
}


func SendMessage(srv *drive.Service, recvId string, sendId string,
	cmdType string, data []byte)(*drive.File, error){
	// 加密数据
	sendData := toolkits.AesEncrypt(data, MessageKey)
	messageMedia := bytes.NewBuffer(sendData)

	// 上传数据
	newFile := &drive.File{
		Name: recvId+".mmd",
		MimeType: "application/octet-stream",
		AppProperties: map[string]string{
			"los":	sendId,
			"owe":	recvId,
			"typ":	cmdType,
		},
	}

	nf, err := srv.Files.Create(newFile).Media(messageMedia).Do()
	return nf, err
}

func SendMessage2(srv *drive.Service, recvId string, sendId string,
	cmdType string, data []byte)(*drive.File, error){
	// 加密数据
	sendData := toolkits.AesEncrypt(data, MessageKey)
	messageMedia := bytes.NewBuffer(sendData)

	// 上传数据
	newFile := &drive.File{
		Name: recvId+".mmd",
		MimeType: "application/octet-stream",
		AppProperties: map[string]string{
			"los":	sendId,
			"owf":	recvId,
			"typ":	cmdType,
		},
	}

	nf, err := srv.Files.Create(newFile).Media(messageMedia).Do()
	return nf, err
}


func ConnectMessage(srv *drive.Service, serverId string, addr string) (*drive.File, error) {
	// 加密数据
	sendData := toolkits.AesEncrypt([]byte(addr), MessageKey)
	messageMedia := bytes.NewBuffer(sendData)

	// 上传数据
	newFile := &drive.File{
		Name: serverId + ".socket",
		MimeType: "application/octet-stream",
		AppProperties: map[string]string{
			"typ":	"s01",
		},
	}

	nf, err := srv.Files.Create(newFile).Media(messageMedia).Do()
	return nf, err
}

func GetOutputData(srv *drive.Service, fileId string, dst io.Writer, errCh chan error) {
	// 循环等待命令数据
	for {
		// 等待数据
		media, err:= srv.Files.Get(fileId).Fields("size, name, appProperties").Do()
		if err != nil {
			println(err.Error())
			errCh <- err
		}
		if media.Size == 0 {
			time.Sleep(time.Duration(1)*time.Second)
			continue
		}

		// 读取数据
		response, err := srv.Files.Get(fileId).Download()
		if err != nil {
			println("Download error:", err)
			errCh <- err
			return
		}
		content, _ := ioutil.ReadAll(response.Body)
		response.Body.Close()
		recvData := toolkits.AesDecrypt(content, MessageKey)
		//println("output", string(recvData))

		dst.Write(recvData)
		//dst.Write([]byte("\r\n"))

		// 清理数据
		messageMedia := bytes.NewBuffer(nil)
		srv.Files.Update(fileId, nil).Media(messageMedia).Do()
	}

}

func GetInputData(srv *drive.Service, fileId string, src io.Reader, errCh chan error) {
	// 循环等待命令数据
	for {
		// 等待数据
		println("[input]begin read")
		buf := make([]byte, 10240)
		n, err := src.Read(buf)
		if err != nil {
			break
		}
		println("[input]read", n, "byte(s)")
		if n == 0 {
			break
		}
		tempMedia := bytes.NewBuffer(nil)
		tempMedia.Write(buf[:n])
		sendData := toolkits.AesEncrypt(tempMedia.Bytes(), toolkits.MessageKey)
		sendMedia := bytes.NewBuffer(sendData)

		// 等待上一次数据被处理
		for {
			media, err := srv.Files.Get(fileId).Fields("size, name, appProperties").Do()
			if err != nil {
				errCh <- err
				return
			}
			if media.Size == 0 {
				srv.Files.Update(fileId, nil).Media(sendMedia).Do()
				break
			}
			time.Sleep(time.Duration(1) * time.Second)
		}
	}
}
