package rdb

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3" //allows 'sqlite' driver in std sqldb lib
)

//RDB is the main database interface.
type RDB struct {
	key        *ecdsa.PrivateKey
	db         *sql.DB
	insertStmt *sql.Stmt
	selectStmt *sql.Stmt
	latestStmt *sql.Stmt
	afterStmt  *sql.Stmt
	beforeStmt *sql.Stmt
}

//Record is an object to keep track
//of some generated bits.
type Record struct {
	ID        int64
	Bits      []byte
	Time      int64
	Hash      []byte
	Signature []byte
}

//MarshalJSON is the custom json marshaler
//for records.
func (r Record) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        int64  `json:"id"`
		Bits      string `json:"bits"`
		Time      int64  `json:"timestamp"`
		Hash      string `json:"hash"`
		Signature string `json:"signature"`
	}{
		r.ID,
		hex.EncodeToString(r.Bits),
		r.Time,
		hex.EncodeToString(r.Hash),
		hex.EncodeToString(r.Signature),
	})
}

//Equals returns whether two records are
//the same.
func (r Record) Equals(r2 Record) bool {
	return r.ID == r2.ID
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

//Open sets up a database at the given file location
//or continues using one that is there if it exists.
func Open(filename string, key string) (rdb RDB, err error) {

	//#TODO think of a better way to hash from a key to a curve
	//maybe also use custom curve generator?
	rdb.key, err = ecdsa.GenerateKey(elliptic.P256(), newStupidReader(key))
	if err != nil {
		return
	}
	// check if database file exists, otherwise make one
	if filename != ":memory:" {
		if _, err = os.Stat(filename); err != nil && os.IsNotExist(err) {
			var f *os.File
			var newDB *sql.DB
			f, err = os.Create(filename)
			if err != nil {
				return
			}
			f.Close()
			newDB, err = sql.Open("sqlite3", filename)
			if err != nil {
				return
			}
			err = newDB.Close()
			if err != nil {
				return
			}
		}
	}
	// open database
	rdb.db, err = sql.Open("sqlite3", filename)
	if err != nil {
		return
	}
	// generate tables
	_, err = rdb.db.Exec(`CREATE TABLE IF NOT EXISTS records (
id INTEGER PRIMARY KEY AUTOINCREMENT,
bits BLOB NOT NULL,
timestamp TIME NOT NULL,
hash BLOB NOT NULL,
signature BLOB NOT NULL
);`)
	// initialize statements
	rdb.insertStmt, err = rdb.db.Prepare("" +
		`INSERT INTO records (bits,timestamp,hash,signature) VALUES (?,?,?,?)`)
	if err != nil {
		return
	}
	rdb.latestStmt, err = rdb.db.Prepare("" +
		`SELECT * FROM records ORDER BY id DESC LIMIT 1`)
	if err != nil {
		return
	}
	rdb.afterStmt, err = rdb.db.Prepare("" +
		`SELECT * FROM records WHERE timestamp>=? ORDER BY timestamp ASC LIMIT 1`)
	if err != nil {
		return
	}
	rdb.beforeStmt, err = rdb.db.Prepare("" +
		`SELECT * FROM records WHERE timestamp<=? ORDER BY timestamp DESC LIMIT 1`)
	if err != nil {
		return
	}
	rdb.selectStmt, err = rdb.db.Prepare("" +
		`SELECT * FROM records WHERE id=? LIMIT 1`)
	if err != nil {
		return
	}
	return
}

//Close destroys the database connection. Future
//attempts to use this will result in an error.
func (rdb RDB) Close() error {
	if rdb.db == nil {
		return errors.New("Connection already closed.")
	}
	return rdb.db.Close()
}

//Select returns the record with the given ID if it
//exists.
func (rdb RDB) Select(id int64) (r Record, err error) {
	err = rdb.selectStmt.QueryRow(id).Scan(&r.ID, &r.Bits, &r.Time, &r.Hash, &r.Signature)
	return
}

//After returns the closest record with time at least
//equal to the given time.
func (rdb RDB) After(time int64) (r Record, err error) {
	err = rdb.afterStmt.QueryRow(time).Scan(&r.ID, &r.Bits, &r.Time, &r.Hash, &r.Signature)
	return
}

//Before returns the closest record with time at most
//equal to the given time.
func (rdb RDB) Before(time int64) (r Record, err error) {
	err = rdb.beforeStmt.QueryRow(time).Scan(&r.ID, &r.Bits, &r.Time, &r.Hash, &r.Signature)
	return
}

//Latest returns the latest record in the database.
func (rdb RDB) Latest() (r Record, err error) {
	err = rdb.latestStmt.QueryRow().Scan(&r.ID, &r.Bits, &r.Time, &r.Hash, &r.Signature)
	return
}

//New generates and returns a new record and saves
//it in the database.
func (rdb RDB) New() (r Record, err error) {
	//fill in new bits
	r.Bits = make([]byte, 64)
	rand.Read(r.Bits)
	//find previous record and hash it's value
	//plus the value of the new bits
	var r0 Record
	r0, err = rdb.Latest()
	var s [32]byte
	if err == sql.ErrNoRows {
		s = sha256.Sum256([]byte(hex.EncodeToString(r.Bits)))
	} else {
		s = sha256.Sum256([]byte(hex.EncodeToString(r0.Hash) +
			hex.EncodeToString(r.Bits)))
	}
	r.Hash = make([]byte, 32)
	for i := 0; i < len(s); i++ {
		r.Hash[i] = s[i]
	}
	//note the time
	r.Time = time.Now().Unix()
	//sign the generated bits and hash
	r.Signature, err = rdb.key.Sign(rand.Reader, append(r.Bits, r.Hash...), nil)
	if err != nil {
		return
	}
	//make an attempt to insert
	//note in order for hash's to be
	//consistent the inserts have to be
	//sequential. Hence the need for the
	//transaction and rollback if it fails
	//to insert the next value.
	tx, err := rdb.db.Begin()
	if err != nil {
		return
	}
	result, err := tx.Stmt(rdb.insertStmt).Exec(r.Bits, r.Time, r.Hash, r.Signature)
	if err != nil {
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return
	}
	r.ID = id
	if id != r0.ID+1 {
		err = tx.Rollback()
		if err != nil {
			return
		}
		err = errors.New("Detected race when trying to insert.")
		return
	}
	err = tx.Commit()
	return
}
