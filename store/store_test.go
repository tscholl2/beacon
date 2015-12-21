package store

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func openTestDB(t *testing.T) RecordStore {
	var rs recordStore
	require.Nil(t, rs.Open("test.db", signer))
	return &rs
}

func closeTestDB(t *testing.T, rs RecordStore) {
	require.Nil(t, rs.Close())
	os.Remove("test.db")
}

func TestOpen(t *testing.T) {
	rs := openTestDB(t)
	rs.Close()
	rs.Open("test.db", signer)
	closeTestDB(t, rs)
}

func TestFirst(t *testing.T) {
	rs := openTestDB(t)
	defer closeTestDB(t, rs)
	_, err := rs.Latest()
	assert.Equal(t, err, ErrNoRecords)
}

func TestNew(t *testing.T) {
	rs := openTestDB(t)
	defer closeTestDB(t, rs)
	for i := 0; i < 3; i++ {
		b := newBitGenerator()()
		r, err := rs.New(b)
		assert.Nil(t, err)
		assert.Equal(t, r.Bits, b)
		r2, err := rs.Latest()
		assert.Nil(t, err)
		assert.True(t, reflect.DeepEqual(r, r2), fmt.Sprintf("args:\nr1=%+v\nr2=%+v", r, r2))
	}
}

func TestSearch(t *testing.T) {
	rs := openTestDB(t)
	defer closeTestDB(t, rs)
	bg := newBitGenerator()
	records := make([]Record, 3)
	times := make([]time.Time, 3)
	var err error
	for i := 0; i < len(times); i++ {
		times[i] = time.Now()
		time.Sleep(time.Millisecond)
		records[i], err = rs.New(bg())
		require.Nil(t, err)
	}
	var r Record
	r, err = rs.After(times[0])
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(records[0], r))
	r, err = rs.After(times[len(times)-1].Add(time.Second))
	assert.Equal(t, err, ErrNoRecords)
	r, err = rs.Before(times[len(times)-1])
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(records[1], r))
	_, err = rs.Before(times[0].Add(-1 * time.Second))
	assert.Equal(t, err, ErrNoRecords)
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
