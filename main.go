package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/tscholl2/beacon/mic"
	"github.com/tscholl2/beacon/store"
)

var (
	rs  = store.NewStore()
	m   *mic.Reader
	key *ecdsa.PrivateKey
)

func init() {
	var filename string
	var err error
	flag.StringVar(&filename, "file", "key.txt",
		"file containing secret key")
	flag.Parse()
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	b := sha512.Sum512(f)
	key, err = ecdsa.GenerateKey(elliptic.P256(), bytes.NewReader(b[:]))
	if err != nil {
		panic(err)
	}
	err = rs.Open("./test.db", key)
	if err != nil {
		panic(err)
	}
	m = new(mic.Reader)
	go func() {
		for {
			var bits [32]byte
			m.Read(bits[:])
			_, err := rs.New(bits)
			if err != nil {
				fmt.Printf("error making record %s\n", err)
			}
			time.Sleep(60 * time.Second)
		}
	}()
}

func get(w http.ResponseWriter, r *http.Request) {
	var rec store.Record
	var err error
	var q string
	q = r.FormValue("before")
	if q != "" {
		var t time.Time
		err = json.Unmarshal([]byte(q), &t)
		if err != nil {
			rec, err = rs.Before(t)
		}
		goto send
	}
	q = r.FormValue("before")
	if q != "" {
		var t time.Time
		err = json.Unmarshal([]byte(q), &t)
		if err != nil {
			rec, err = rs.After(t)
		}
		goto send
	}
	rec, err = rs.Latest()
send:
	json.NewEncoder(w).Encode(struct {
		R store.Record `json:"record"`
		E error        `json:"error,omitempty"`
	}{rec, err})
}

func getAudio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=\"raw.wav\"")
	if r.FormValue("raw") == "" {
		w.Header().Set("Content-Type", "audio/basic")
		h := []byte{82, 73, 70, 70, 100, 31, 0, 0, 87, 65,
			86, 69, 102, 109, 116, 32, 16, 0, 0, 0, 1, 0, 1,
			0, 64, 31, 0, 0, 64, 31, 0, 0, 1, 0, 8, 0, 100, 97,
			116, 97, 64, 31, 0, 0}
		w.Header().Set("Content-Length",
			fmt.Sprintf("%d", len(h)+len(m.LastSample)))
		w.Write(h)
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename=\"raw.wav\"")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length",
			fmt.Sprintf("%d", len(m.LastSample)))
	}
	w.Write(m.LastSample[:])
}

func getKey(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("raw") == "" {
		b, err := x509.MarshalPKIXPublicKey(key.Public())
		json.NewEncoder(w).Encode(struct {
			K string `json:"key"`
			E error  `json:"error,omitempty"`
		}{base64.StdEncoding.EncodeToString(b), err})
	} else {
		json.NewEncoder(w).Encode(key.Public())
	}
}

func headers(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func main() {
	http.HandleFunc("/", get)
	http.HandleFunc("/audio", getAudio)
	http.HandleFunc("/key", getKey)
	http.ListenAndServe(":8888", headers(http.DefaultServeMux))
}
