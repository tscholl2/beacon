package beacon

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	signer *testSigner
	reader *testReader
	bitGen *testReader
)

func init() {
	// Sample key generated with
	// http://play.golang.org/p/SYdIwEX5oj
	buf, _ := base64.StdEncoding.DecodeString(`MGgCAQEEHGVPTAPaVNX4YmbVyfTYW8FWJMpZCo2R9HChDH6gBwYFK4EEACGhPAM6AAQU2Bbh+7LCIFwTYas4CZyPJ1QKL9hnWGFPTrpF4LawRo8j40KLu0A9ickIzi0dCN+CrA9gxWO8yA==`)
	key, _ := x509.ParseECPrivateKey(buf)
	reader = &testReader{rand.NewSource(123)}
	signer = &testSigner{key}
}

func openTestDB() RecordStore {
	var rs recordStore
	err := rs.Open("test.db", signer)
	if err != nil {
		panic(err)
	}
	return &rs
}

func closeTestDB(rs RecordStore) {
	err := rs.Close()
	if err != nil {
		panic(err)
	}
	os.Remove("test.db")
}

func TestOpen(t *testing.T) {
	rs := openTestDB()
	rs.Close()
	rs.Open("test.db", signer)
	closeTestDB(rs)
}

func TestFirst(t *testing.T) {
	rs := openTestDB()
	defer closeTestDB(rs)
	_, err := rs.Latest()
	if err != ErrNoRecords {
		t.Errorf("unexpected error\n%s", err)
	}
}

func TestNew(t *testing.T) {
	rs := openTestDB()
	defer closeTestDB(rs)
	for i := 0; i < 3; i++ {
		b := newBitGenerator()()
		r, err := rs.New(b)
		if err != nil {
			t.Errorf("unexpected error\n%s", err)
		}
		if r.Bits != b {
			t.Errorf("expected %x got %x", b, r.Bits)
		}
		r2, err := rs.Latest()
		if err != nil {
			t.Errorf("unexpected error\n%s", err)
		}
		if !reflect.DeepEqual(r, r2) {
			t.Errorf("expected %+v got %+v", r, r2)
		}
	}
}

func TestSearch(t *testing.T) {
	rs := openTestDB()
	defer closeTestDB(rs)
	bg := newBitGenerator()
	records := make([]Record, 3)
	times := make([]time.Time, 3)
	for i := 0; i < 3; i++ {
		times[i] = time.Now()
		time.Sleep(time.Millisecond)
		records[i], _ = rs.New(bg())
	}
	var r Record
	var err error
	r, err = rs.After(times[0])
	if err != nil {
		t.Errorf("unexpected error\n%s", err)
	}
	if !reflect.DeepEqual(records[0], r) {
		t.Errorf("expected %+v got %+v", records[0], r)
	}
	r, err = rs.After(times[2].Add(time.Second))
	if err != ErrNoRecords {
		t.Errorf("unexpected error\n%s", err)
	}
	r, err = rs.Before(times[2])
	if err != nil {
		t.Errorf("unexpected error\n%s", err)
	}
	if !reflect.DeepEqual(records[1], r) {
		t.Errorf("expected %+v got %+v", records[1], r)
	}
	_, err = rs.Before(times[0])
	if err != ErrNoRecords {
		t.Errorf("unexpected error\n%s", err)
	}
}

/*
	utilities for deterministic tests
*/

// returns a function which generates bits
// randomly but with a set seed.
func newBitGenerator() func() [32]byte {
	reader := testReader{rand.NewSource(100)}
	return func() [32]byte {
		var bits [32]byte
		reader.Read(bits[:])
		return bits
	}
}

// testSigner uses custom random source
type testSigner struct {
	key *ecdsa.PrivateKey
}

func (s *testSigner) Public() crypto.PublicKey {
	return s.key.Public()
}
func (s *testSigner) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	return s.key.Sign(reader, msg, opts)
}

// testReader reads from a random source
// whose seed is set above
type testReader struct {
	source rand.Source
}

func (r *testReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(r.source.Int63())
	}
	return len(p), nil
}
