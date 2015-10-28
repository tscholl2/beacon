package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha512"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/tscholl2/beacon"
)

var (
	store beacon.RecordStore
)

func init() {
	var s string
	flag.StringVar(&s, "key", "abc-123", "string used to generate a private key")
	flag.Parse()
	b := sha512.Sum512([]byte(s))
	key, err := ecdsa.GenerateKey(elliptic.P256(), bytes.NewReader(b[:]))
	if err != nil {
		panic(err)
	}
	store := beacon.NewStore()
	err = store.Open("./test.db", key)
	if err != nil {
		panic(err)
	}
	m := new(mic)
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

func middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ...do something...
		h.ServeHTTP(w, r)
	})
}

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello")
}

func main() {

	h := contextHandler(middleware(http.HandlerFunc(handler)))
	http.ListenAndServe(":8080", h)
}
