package store

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"time"

	_ "github.com/mattn/go-sqlite3" // allows 'sqlite' driver in std sql package
)

var (
	recordBucket = []byte("records")
)

type recordStore struct {
	db     *sql.DB
	signer crypto.Signer
}

// NewStore returns something which
// implements the store interface.
func NewStore() RecordStore {
	return &recordStore{}
}

func (rs *recordStore) Open(filename string, signer crypto.Signer) (err error) {
	rs.signer = signer
	// check if database file exists, otherwise make one
	// if inmemory file than make new db in memory
	rs.db, err = sql.Open("sqlite3", filename)
	if err != nil {
		return
	}
	_, err = rs.db.Exec(`
	CREATE TABLE IF NOT EXISTS records (
		id				INTEGER PRIMARY KEY NOT NULL,
		bits			BLOB NOT NULL,
		time 			TIMESTAMP NOT NULL,
		hash 			BLOB NOT NULL,
		signature BLOB NOT NULL
	);`)
	return
}

func (rs *recordStore) Close() error {
	return rs.db.Close()
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

// extractRecord assumes that the sql call
// pulled the columns in the order:
// `SELECT id,bits,time,hash,signature FROM records`
func extractRecord(scanner rowScanner) (r Record, err error) {
	var b, h []byte
	if err = scanner.Scan(&r.ID, &b, &r.Time, &h, &r.Signature); err == sql.ErrNoRows {
		err = ErrNoRecords
	}
	for i := 0; i < len(r.Bits) && i < len(b) && i < len(h); i++ {
		r.Bits[i] = b[i] // assume bits and hash have same length
		r.Hash[i] = h[i]
	}
	return
}

func (rs *recordStore) Latest() (r Record, err error) {
	return extractRecord(rs.db.QueryRow(`
		SELECT id,bits,time,hash,signature FROM records
		WHERE id=(SELECT MAX(id) FROM records)
	`))
}

func (rs *recordStore) After(min time.Time) (r Record, err error) {
	return extractRecord(rs.db.QueryRow(`
		SELECT id,bits,time,hash,signature FROM records
		WHERE id=(SELECT MIN(id) FROM records WHERE time>=?)
	`, min.UTC()))
}

func (rs *recordStore) Before(max time.Time) (r Record, err error) {
	return extractRecord(rs.db.QueryRow(`
		SELECT id,bits,time,hash,signature FROM records
		WHERE id=(SELECT MAX(id) FROM records WHERE time<=?)
	`, max.UTC()))
}

func (rs *recordStore) New(bits [32]byte) (r Record, err error) {
	tx, err := rs.db.Begin()
	if err != nil {
		return
	}
	defer tx.Commit()
	old, err := extractRecord(tx.QueryRow(`
		SELECT id,bits,time,hash,signature FROM records
		WHERE id=(SELECT MAX(id) FROM records)
	`))
	if err != nil && err != ErrNoRecords {
		return
	}
	r.Bits = bits
	if err == ErrNoRecords {
		// this is the first record
		b, err := x509.MarshalPKIXPublicKey(rs.signer.Public())
		if err != nil {
			return r, err
		}
		pubKey := base64.StdEncoding.EncodeToString(b)
		r.Hash = sha256.Sum256([]byte(pubKey + base64.StdEncoding.EncodeToString(r.Bits[:])))
	} else {
		// this is not the first record
		r.Hash = sha256.Sum256([]byte(base64.StdEncoding.EncodeToString(old.Bits[:]) + base64.StdEncoding.EncodeToString(r.Bits[:])))
	}
	// note the time
	r.Time = time.Now().UTC()
	// sign the generated bits and hash together
	r.Signature, err = rs.signer.Sign(rand.Reader, append(r.Bits[:], r.Hash[:]...), nil)
	if err != nil {
		return
	}
	// ...read or write...
	result, err := tx.Exec(`
		INSERT INTO records (bits,time,hash,signature) VALUES (?,?,?,?)
	`, r.Bits[:], r.Time, r.Hash[:], r.Signature[:])
	if err != nil {
		return
	}
	i, _ := result.LastInsertId()
	r.ID = uint64(i)
	return
}
