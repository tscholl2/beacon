package main

import (
	"crypto/sha512"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/cocoonlife/goalsa"
	"github.com/tscholl2/beacon/rdb"
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/middleware"
)

var (
	records rdb.RDB
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
	return
}

func init() {
	//#TODO read in key from file
	var err error
	records, err = rdb.Open("./test.db", "key", new(mic))
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			records.New()
			time.Sleep(10 * time.Second)
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

func send(rec rdb.Record, err error, w http.ResponseWriter) {
	//if record.Time not set then assume none found
	if rec.Time == 0 && err != nil {
		err = errors.New("No record found.")
	}
	if err != nil {
		b, _ := json.Marshal(struct {
			E string `json:"error"`
		}{err.Error()})
		w.Write(b)
		return
	}
	b, _ := json.Marshal(struct {
		R rdb.Record `json:"record"`
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
		send(rdb.Record{}, err, w)
		return
	}
	rec, err := records.Select(id)
	send(rec, err, w)
	return
}

func getBefore(c web.C, w http.ResponseWriter, r *http.Request) {
	t, err := strconv.ParseInt(c.URLParams["time"], 10, 64)
	if err != nil {
		send(rdb.Record{}, err, w)
		return
	}
	rec, err := records.Before(t)
	send(rec, err, w)
	return
}

func getAfter(c web.C, w http.ResponseWriter, r *http.Request) {
	t, err := strconv.ParseInt(c.URLParams["time"], 10, 64)
	if err != nil {
		send(rdb.Record{}, err, w)
		return
	}
	rec, err := records.After(t)
	send(rec, err, w)
	return
}

func main() {
	defer records.Close()           //#TODO this doesn't get run? how to handle exit?
	goji.Abandon(middleware.Logger) //comment out to see log
	goji.Use(headers)
	goji.Get("/", getLatest)
	goji.Get("/:id", getID)
	goji.Get("/before/:time", getBefore)
	goji.Get("/after/:time", getAfter)
	goji.Serve()
}
