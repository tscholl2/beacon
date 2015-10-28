package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha512"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/tscholl2/beacon"
)

var (
	store beacon.RecordStore
	m     *mic
	key   *ecdsa.PrivateKey
)

func init() {
	var s string
	var err error
	flag.StringVar(&s, "key", "abc-123", "string used to generate a private key")
	flag.Parse()
	b := sha512.Sum512([]byte(s))
	key, err = ecdsa.GenerateKey(elliptic.P256(), bytes.NewReader(b[:]))
	if err != nil {
		panic(err)
	}
	store = beacon.NewStore()
	err = store.Open("./test.db", key)
	if err != nil {
		panic(err)
	}
	m = new(mic)
	go func() {
		for {
			var bits [32]byte
			m.Read(bits[:])
			_, err := store.New(bits)
			if err != nil {
				fmt.Printf("error making record %s\n", err)
			}
			time.Sleep(60 * time.Second)
		}
	}()
}

func get(w http.ResponseWriter, r *http.Request) {
	var rec beacon.Record
	var err error
	var q string
	q = r.FormValue("before")
	if q != "" {
		var t time.Time
		err = json.Unmarshal([]byte(q), &t)
		if err != nil {
			rec, err = store.Before(t)
		}
		goto send
	}
	q = r.FormValue("before")
	if q != "" {
		var t time.Time
		err = json.Unmarshal([]byte(q), &t)
		if err != nil {
			rec, err = store.After(t)
		}
		goto send
	}
	rec, err = store.Latest()
send:
	json.NewEncoder(w).Encode(struct {
		R beacon.Record `json:"record"`
		E error         `json:"error,omitempty"`
	}{rec, err})
}

func getRaw(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=\"raw.wav\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(m.lastSample)))
	w.Write(m.lastSample[:])
}

func getAudio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=\"raw.wav\"")
	w.Header().Set("Content-Type", "audio/basic")
	h := []byte{82, 73, 70, 70, 100, 31, 0, 0, 87, 65, 86, 69, 102, 109, 116, 32, 16, 0, 0, 0, 1, 0, 1, 0, 64, 31, 0, 0, 64, 31, 0, 0, 1, 0, 8, 0, 100, 97, 116, 97, 64, 31, 0, 0}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(h)+len(m.lastSample)))
	w.Write(append(h, m.lastSample[:]...))
}

func getKey(w http.ResponseWriter, r *http.Request) {
	/*
		b, err := x509.MarshalPKIXPublicKey(key.Public())
		if err != nil {
			json.NewEncoder(w).Encode(struct {
				E error `json:"error"`
			}{err})
			return
		}
	*/
	json.NewEncoder(w).Encode(key.Public())
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
	http.HandleFunc("/raw", getRaw)
	http.HandleFunc("/audio", getAudio)
	http.HandleFunc("/key", getKey)
	http.ListenAndServe(":8888", headers(http.DefaultServeMux))
}
