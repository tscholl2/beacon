package beacon

import (
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// ErrNoRecords is returned by any reading method
// which may not find any records.
var ErrNoRecords = errors.New("store: no record found")

// Record is an object to keep track
// of some generated bits, and all
// associated meta data.
type Record struct {
	ID        uint64
	Bits      [32]byte
	Time      time.Time
	Hash      [32]byte
	Signature []byte
}

// MarshalJSON is custom json marshaler
// for records. Uses base64 encoding of
// bytes.
func (r Record) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		I uint64    `json:"id"`
		B string    `json:"bits"`
		T time.Time `json:"time"`
		H string    `json:"hash"`
		S string    `json:"signature"`
	}{r.ID, base64.StdEncoding.EncodeToString(r.Bits[:]), r.Time, base64.StdEncoding.EncodeToString(r.Hash[:]), base64.StdEncoding.EncodeToString(r.Signature[:])})
}

// RecordStore is the store to access
// records.
type RecordStore interface {
	Open(filename string, signer crypto.Signer) error
	Close() error
	New(bits [32]byte) (Record, error)
	Latest() (Record, error)
	After(time.Time) (Record, error)
	Before(time.Time) (Record, error)
}
