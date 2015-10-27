package beacon

import (
	"crypto"
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
