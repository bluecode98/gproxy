package climanage

import (
	"../toolkits"
	"os"
	"io/ioutil"
	"encoding/json"
	"golang.org/x/net/context"
	"google.golang.org/api/drive/v3"
	"golang.org/x/oauth2"
	"time"
)

// Google login key
type LoginInfo struct {
	ClientID      	string  `json:"ClientID"`
	ClientSecret    string  `json:"ClientSecret"`
	RedirectURL    	string  `json:"RedirectURL"`
	AccessToken    	string  `json:"AccessToken"`
	RefreshToken    string  `json:"RefreshToken"`
}

var configKey = "123456781234567812345678"

func EncodeConfig(configFile string) ([]byte, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	configData, err := ioutil.ReadAll(file)
	encodeData := toolkits.AesEncrypt(configData, configKey)
	return encodeData, nil
}

func DecodeConfig(configFile string) ([]byte, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	configData, err := ioutil.ReadAll(file)
	decodeData := toolkits.AesDecrypt(configData, configKey)
	return decodeData, nil
}


func LoginDriveFromFile(configFile string) (*drive.Service, error) {
	// load login info from json file
	configData, err := DecodeConfig(configFile)
	if err != nil {
		return nil, err
	}

	loginInfo := &LoginInfo{}
	err = json.Unmarshal(configData, loginInfo)
	if err != nil {
		return nil, err
	}

	// login
	config := &oauth2.Config{
		ClientID:     loginInfo.ClientID,
		ClientSecret: loginInfo.ClientSecret,
		RedirectURL:  loginInfo.RedirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/drive"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
	}

	token := &oauth2.Token{
		AccessToken:	loginInfo.AccessToken,
		TokenType:		"Bearer",
		RefreshToken:	loginInfo.RefreshToken,
		Expiry: 		time.Now(),
	}
	client := config.Client(context.Background(), token)

	srv, err := drive.New(client)
	if err != nil {
		return nil, err
	}

	return srv, nil
}
