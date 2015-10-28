package beacon

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"time"

	"github.com/boltdb/bolt"
)

var (
	recordBucket = []byte("records")
)

type recordStore struct {
	db     *bolt.DB
	signer crypto.Signer
}

func NewStore() RecordStore {
	return &recordStore{}
}

func (rs *recordStore) Open(filename string, signer crypto.Signer) (err error) {
	rs.signer = signer
	rs.db, err = bolt.Open(filename, 0666, nil)
	if err != nil {
		return
	}
	return rs.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(recordBucket)
		return err
	})
}

func (rs *recordStore) Close() error {
	return rs.db.Close()
}

func (rs *recordStore) Latest() (r Record, err error) {
	return r, rs.db.View(func(tx *bolt.Tx) error {
		k, v := tx.Bucket(recordBucket).Cursor().Last()
		if k == nil || v == nil {
			return ErrNoRecords
		}
		return gob.NewDecoder(bytes.NewReader(v)).Decode(&r)
	})
}

func (rs *recordStore) After(min time.Time) (r Record, err error) {
	return r, rs.db.View(func(tx *bolt.Tx) error {
		buf, _ := json.Marshal(min)
		c := tx.Bucket(recordBucket).Cursor()
		k, v := c.Seek(buf)
		if k == nil {
			k, v = c.Prev()
		}
		if k == nil || bytes.Compare(k, buf) == -1 {
			return ErrNoRecords
		}
		return gob.NewDecoder(bytes.NewReader(v)).Decode(&r)
	})
}

func (rs *recordStore) Before(max time.Time) (r Record, err error) {
	return r, rs.db.View(func(tx *bolt.Tx) error {
		buf, _ := json.Marshal(max)
		c := tx.Bucket(recordBucket).Cursor()
		k, v := c.Seek(buf)
		if k == nil || bytes.Compare(k, buf) == 1 {
			k, v = c.Prev()
		}
		if k == nil || bytes.Compare(k, buf) == 1 {
			return ErrNoRecords
		}
		return gob.NewDecoder(bytes.NewReader(v)).Decode(&r)
	})
}

func (rs *recordStore) New(bits [32]byte) (r Record, err error) {
	err = rs.db.Update(func(tx *bolt.Tx) error {
		// read latest record
		old, err := rs.Latest()
		if err != nil && err != ErrNoRecords {
			return err
		}
		r.Bits = bits
		if err == ErrNoRecords {
			// this is the first record
			b, err := x509.MarshalPKIXPublicKey(rs.signer.Public())
			if err != nil {
				return err
			}
			pubKey := base64.StdEncoding.EncodeToString(b)
			r.Hash = sha256.Sum256([]byte(pubKey + base64.StdEncoding.EncodeToString(r.Bits[:])))
		} else {
			// this is not the first record
			r.Hash = sha256.Sum256([]byte(base64.StdEncoding.EncodeToString(old.Bits[:]) + base64.StdEncoding.EncodeToString(r.Bits[:])))
		}
		// note the time
		r.Time = time.Now()
		// sign the generated bits and hash together
		r.Signature, err = rs.signer.Sign(rand.Reader, append(r.Bits[:], r.Hash[:]...), nil)
		if err != nil {
			return err
		}
		// ...read or write...
		b := tx.Bucket(recordBucket)
		r.ID, _ = b.NextSequence()
		buf := &bytes.Buffer{}
		err = gob.NewEncoder(buf).Encode(r)
		if err != nil {
			return err
		}
		k, _ := json.Marshal(r.Time)
		return b.Put(k, buf.Bytes())
	})
	return
}
