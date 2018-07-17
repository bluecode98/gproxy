package main

import (
	"github.com/op/go-logging"
	"os"
	"net/http"
	"fmt"
	"encoding/json"
	"crypto/tls"
)

var log = logging.MustGetLogger("goserver")
var format = logging.MustStringFormatter(
	//`%{time:15:04:05.000} %{shortfunc} > %{level:s} %{message}`,
	`%{time:15:04:05.000} > %{level:s} %{message}`,
)

func main() {
	// init log config
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format)
	logging.SetBackend(backend2Formatter)

	httpMux := http.NewServeMux()

	httpMux.HandleFunc("/", defaultHandler )
	httpMux.HandleFunc("/h", msgHandler)

	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.CurveP521,
			tls.CurveP384,
			tls.CurveP256,
			},
		PreferServerCipherSuites:true,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			},
		}


	srv := &http.Server{
		Addr: "0.0.0.0:8081",
		Handler: httpMux,
		TLSConfig:cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}

	log.Debug("listening...")
	go srv.ListenAndServeTLS("ssl\\bluenet.crt", "ssl\\bluenet.key")
	http.ListenAndServe(":8080", httpMux)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	newReq := map[string]string{
		"method": r.Method,
		"path": r.RequestURI,
		"cookie": r.Header.Get("Cookie"),
		"form": r.Form.Encode(),
	}
	d, _ := json.Marshal(newReq)
	log.Debug("proxy", string(d))
	fmt.Fprint(w, "200")
}

func msgHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	id := r.Form["id"]
	//id := r.FormValue("id")
	//fmt.Fprint(w, r.RequestURI)
	if len(id)>0 {
		fmt.Fprint(w, "id:", id[0])
	} else {
		fmt.Fprint(w, "not find id")
	}
}

