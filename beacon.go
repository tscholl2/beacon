package beacon

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3" //allows 'sqlite' driver in std sqldb lib
)

//RDB is the main database interface.
type RDB struct {
	signer     crypto.Signer
	reader     io.Reader
	db         *sql.DB
	insertStmt *sql.Stmt
	selectStmt *sql.Stmt
	latestStmt *sql.Stmt
	afterStmt  *sql.Stmt
	beforeStmt *sql.Stmt
	//EncodedPublicKey is the base64
	//PKIX encoding the public key used
	//to do the signing.
	EncodedPublicKey string
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
		ID int64  `json:"id"`
		B  string `json:"bits"`
		T  int64  `json:"timestamp"`
		H  string `json:"hash"`
		S  string `json:"signature"`
	}{
		r.ID,
		base64.StdEncoding.EncodeToString(r.Bits),
		r.Time,
		base64.StdEncoding.EncodeToString(r.Hash),
		base64.StdEncoding.EncodeToString(r.Signature),
	})
}

//Equals returns whether two records are
//the same.
func (r Record) Equals(r2 Record) bool {
	return r.ID == r2.ID
}

//Open sets up a database at the given file location
//or continues using one that is there if it exists.
//signature is the private key to sign with
//and rand is the source of randomness for the bytes.
func Open(filename string, signer crypto.Signer, rand io.Reader) (rdb RDB, err error) {
	rdb.reader = rand
	rdb.signer = signer
	b, _ := x509.MarshalPKIXPublicKey(signer.Public())
	rdb.EncodedPublicKey = base64.StdEncoding.EncodeToString(b)
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
	n, err := io.ReadFull(rdb.reader, r.Bits)
	if err != nil {
		return
	}
	if n < 64 {
		err = errors.New("Unable to fill up random bits.")
		return
	}
	//find previous record and hash it's value
	//plus the value of the new bits
	var r0 Record
	r0, err = rdb.Latest()
	var s [32]byte
	if err == sql.ErrNoRows {
		s = sha256.Sum256([]byte(rdb.EncodedPublicKey +
			base64.StdEncoding.EncodeToString(r.Bits)))
	} else {
		s = sha256.Sum256([]byte(base64.StdEncoding.EncodeToString(r0.Bits) +
			base64.StdEncoding.EncodeToString(r.Bits)))
	}
	r.Hash = make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		r.Hash[i] = s[i]
	}
	//note the time
	r.Time = time.Now().Unix()
	//sign the generated bits and hash together
	r.Signature, err = rdb.signer.Sign(
		rand.Reader, append(r.Bits, r.Hash...), nil)
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
