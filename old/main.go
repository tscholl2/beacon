package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"net/http"
	"strconv"
	"time"

	"github.com/cocoonlife/goalsa"
	"github.com/tscholl2/beacon"
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
)

var (
	records  beacon.RDB
	key      ecdsa.PrivateKey
	rawAudio []byte
)

type mic struct{}

func min(a int, b int) int {
	if a <= b {
		return a
	}
	return b
}

func (*mic) Read(p []byte) (n int, err error) {
	dev, err := alsa.NewCaptureDevice("default", 1, alsa.FormatU8, 8000, alsa.BufferParams{})
	if err != nil {
		return
	}
	b1 := make([]int8, 8000)
	_, err = dev.Read(b1)
	if err != nil {
		return
	}
	b2 := make([]byte, len(b1))
	for i := 0; i < len(b1); i++ {
		b2[i] = byte(b1[i])
	}
	dev.Close()
	c := sha512.Sum512(b2)
	for n = 0; n < min(len(b1), len(p)); n++ {
		p[n] = c[n]
	}
	rawAudio = b2
	return
}

type stupidReader struct {
	i int
	h hash.Hash
	a []byte
}

func newStupidReader(key string) *stupidReader {
	var s stupidReader
	s.h = sha512.New()
	s.h.Write([]byte(key))
	s.a = s.h.Sum([]byte{})
	return &s
}
func (s *stupidReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = s.a[s.i]
		s.i = s.i + 1
		if s.i == len(s.a) {
			s.i = 0
			s.h.Write(s.a)
			s.a = s.h.Sum([]byte{})
		}
	}
	return len(p), nil
}

func init() {
	//#TODO read in key from file
	//#TODO think of a better way to hash from a key to a curve
	//maybe also use custom curve generator?
	var s string
	flag.StringVar(&s, "secret", "", "String used to generate a private key. Must not be ''.")
	flag.Parse()
	if s == "" {
		panic(errors.New("'secret' argument must not be ''. Run with -help"))
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), newStupidReader(s))
	if err != nil {
		panic(err)
	}
	records, err = beacon.Open("./test.db", key, new(mic))
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			_, err := records.New()
			if err != nil {
				fmt.Errorf("Error making record: %s", err)
			}
			time.Sleep(60 * time.Second)
		}
	}()
}

func headers(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func send(rec beacon.Record, err error, w http.ResponseWriter) {
	//if record.Time not set then assume none found
	if rec.Time == 0 && err != nil {
		err = errors.New("No record found.")
	}
	if err != nil {
		fmt.Errorf("Error getting record: %s", err)
		b, _ := json.Marshal(struct {
			E string `json:"error"`
		}{err.Error()})
		w.Write(b)
		return
	}
	b, _ := json.Marshal(struct {
		R beacon.Record `json:"record"`
	}{rec})
	w.Write(b)
	return
}

func getLatest(w http.ResponseWriter, r *http.Request) {
	rec, err := records.Latest()
	send(rec, err, w)
	return
}

func getID(c web.C, w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(c.URLParams["id"], 10, 64)
	if err != nil || id == 0 {
		send(beacon.Record{}, err, w)
		return
	}
	rec, err := records.Select(id)
	send(rec, err, w)
	return
}

func getBefore(c web.C, w http.ResponseWriter, r *http.Request) {
	t, err := strconv.ParseInt(c.URLParams["time"], 10, 64)
	if err != nil {
		send(beacon.Record{}, err, w)
		return
	}
	rec, err := records.Before(t)
	send(rec, err, w)
	return
}

func getAfter(c web.C, w http.ResponseWriter, r *http.Request) {
	t, err := strconv.ParseInt(c.URLParams["time"], 10, 64)
	if err != nil {
		send(beacon.Record{}, err, w)
		return
	}
	rec, err := records.After(t)
	send(rec, err, w)
	return
}

func getKey(w http.ResponseWriter, r *http.Request) {
	b, _ := json.Marshal(struct {
		K string `json:"key"`
	}{records.EncodedPublicKey})
	w.Write(b)
	return
}

func getRaw(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=\"raw.wav\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(rawAudio)))
	w.Write(rawAudio)
	return
}

func getAudio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=\"raw.wav\"")
	w.Header().Set("Content-Type", "audio/basic")
	h := []byte{82, 73, 70, 70, 100, 31, 0, 0, 87, 65, 86, 69, 102, 109, 116, 32, 16, 0, 0, 0, 1, 0, 1, 0, 64, 31, 0, 0, 64, 31, 0, 0, 1, 0, 8, 0, 100, 97, 116, 97, 64, 31, 0, 0}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(h)+len(rawAudio)))
	w.Write(append(h, rawAudio...))
	return
}

func main() {
	goji.Use(headers)
	goji.Get("/", getLatest)
	goji.Get("/raw", getRaw)
	goji.Get("/audio", getAudio)
	goji.Get("/key", getKey)
	goji.Get("/:id", getID)
	goji.Get("/before/:time", getBefore)
	goji.Get("/after/:time", getAfter)
	goji.Serve()
}
