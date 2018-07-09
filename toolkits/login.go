package toolkits

import (
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

// cta
func LoginDrive() (*drive.Service, error) {
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
		return nil, err
	}

	return srv, nil
}

// yue
//func LoginDrive() (*drive.Service, error) {
//	// login
//	config := &oauth2.Config{
//		ClientID:     "531233084365-6tjqidobh1huopnp795cjm988q2rgu98.apps.googleusercontent.com",
//		ClientSecret: "DqjWf6Iim9DolBXcelx4uJwo",
//		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
//		Scopes:       []string{"https://www.googleapis.com/auth/drive"},
//		Endpoint: oauth2.Endpoint{
//			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
//			TokenURL: "https://accounts.google.com/o/oauth2/token",
//		},
//	}
//
//	token := &oauth2.Token{
//		AccessToken:"ya29.GlvdBYwaXwT005MZD0zKpmRLwe6zAJgTt9jOKrnk-6toF9nDsZ5QrVuSfLY5BQHtFeBlou8W0EM37bgsfQMLP7NYxbp2k4cc1f2FZAqFqI0CrM4uEWjEWJKRfVs-",
//		TokenType:"Bearer",
//		RefreshToken:"1/UFsfSFBH7G4F9lpgvYZ1sAZEizu4LB2OAnSXvFENjBw",
//		Expiry: time.Now(),
//	}
//	client := config.Client(context.Background(), token)
//
//	srv, err := drive.New(client)
//	if err != nil {
//		return nil, err
//	}
//
//	return srv, nil
//}
