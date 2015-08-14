package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tscholl2/beacon/rdb"
)

var (
	records rdb.RDB
)

func init() {
	//#TODO read in key from file
	var err error
	records, err = rdb.Open("./test.db", "key")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			records.New()
			time.Sleep(60 * time.Second)
		}
	}()
}

type response struct {
	Record rdb.Record `json:"record"`
	Error  string     `json:"error,omitempty"`
}

func readErr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func getLatest(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)
	rec, err := records.Latest()
	b, _ := json.Marshal(&response{rec, readErr(err)})
	w.Write(b)
	return
}

//#TODO write other API calls

func main() {
	defer records.Close() //#TODO this doesn't get run? how to handle exit?
	http.HandleFunc("/latest", getLatest)
	http.ListenAndServe(":8888", nil)
}
