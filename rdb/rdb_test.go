package rdb

import (
	"crypto/rand"
	"os"
	"testing"
	"time"
)

func TestInitialize(t *testing.T) {
	fn := ".hiddentestingdatabasefile"
	rdb, err := Open(fn, "secret", rand.Reader)
	if err != nil {
		t.Errorf("Err initializing1:\n\t%s", err)
	}
	_, err = rdb.New()
	if err != nil {
		t.Errorf("Err inserting new:\n\t%s", err)
	}
	err = rdb.Close()
	if err != nil {
		t.Errorf("Err closing rdb1:\n\t%s", err)
	}
	//open again to make sure it worked and saved
	rdb, err = Open(fn, "secret", rand.Reader)
	if err != nil {
		t.Errorf("Err initializing2:\n\t%s", err)
	}
	r, err := rdb.Latest()
	if err != nil {
		t.Errorf("Err reading value:\n\t%s", err)
	}
	if r.ID != 1 {
		t.Errorf("Err reading value: record not 1")
	}
	err = rdb.Close()
	if err != nil {
		t.Errorf("Err closing rdb2:\n\t%s", err)
	}
	//clean up
	err = os.Remove(fn)
	if err != nil {
		t.Errorf("Err removing database file, please remove './%s' manually.", fn)
	}
}

func TestOpen(t *testing.T) {
	rdb, err := Open(":memory:", "secret", rand.Reader)
	if err != nil {
		t.Errorf("Err initializing:\n\t%s", err)
	}
	defer rdb.Close()
	for i := 0; i < 10; i++ {
		_, err := rdb.New()
		if err != nil {
			t.Errorf("Err generating record:\n\t%s", err)
		}
	}
}

func TestSelects(t *testing.T) {
	rdb, err := Open(":memory:", "secret", rand.Reader)
	if err != nil {
		t.Errorf("Err initializing:\n\t%s", err)
	}
	defer rdb.Close()
	R := make([]Record, 3)
	for i := 0; i < len(R); i++ {
		R[i], err = rdb.New()
		time.Sleep(time.Second)
		if err != nil {
			t.Errorf("Err generating record:\n\t%s", err)
		}
	}
	var r Record
	r, err = rdb.Latest()
	if err != nil {
		t.Errorf("Err on 'latest':\n\t%s", err)
	}
	if !r.Equals(R[len(R)-1]) {
		t.Errorf("Err on 'latest'. Got:\n\t%+v\nWant:\n\t%+v", r, R[len(R)-1])
	}
	r, err = rdb.Select(3)
	if err != nil {
		t.Errorf("Err on 'select':\n\t%s", err)
	}
	if err != nil {
		t.Errorf("Err on 'select'. Got:\n\t%+v\nWant:\n\t%+v", r, R[2])
	}
	r, err = rdb.After(R[2].Time)
	if err != nil {
		t.Errorf("Err on 'after':\n\t%s", err)
	}
	if !r.Equals(R[2]) {
		t.Errorf("Err on 'after'. Got:\n\t%+v\nWant:\n\t%+v", r, R[2])
	}
	r, err = rdb.Before(R[1].Time)
	if err != nil {
		t.Errorf("Err on 'before':\n\t%s", err)
	}
	if !r.Equals(R[1]) {
		t.Errorf("Err on 'before'. Got:\n\t%+v\nWant:\n\t%+v", r, R[1])
	}
}
