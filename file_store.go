// Copyright (c), Ruslan Sendecky. All rights reserved.
// Use of this source code is governed by the MIT license.
// See the LICENSE file in the project root for more information.

package gsession

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"

	"github.com/dgraph-io/badger"
)

// FileStore struct.
type FileStore struct {
	shelf *badger.DB
}

// NewFileStore creates a new file store.
// Takes directory path for the database files and ticker period.
// Ticker sets duration for how often expired sessions are cleaned up.
// Empty directory string defaults to "session".
func NewFileStore(dir string, tic time.Duration) *FileStore {
	if dir == "" {
		dir = "session"
	}
	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	store := &FileStore{
		shelf: db,
	}
	go store.vacuum(5)
	go store.expire(tic)
	return store
}

// Create adds a new session entry to the store.
// Takes a session ID and session expiry duration.
func (s *FileStore) Create(id string, exp time.Duration) (err error) {
	ses := Session{
		Expiry: time.Now().Add(exp),
		Tstamp: time.Now(),
		Token:  "",
		Data:   make(map[string]interface{}),
	}
	err = s.shelf.Update(func(txn *badger.Txn) error {
		bts, err := encGob(ses)
		if err != nil {
			return err
		}
		err = txn.Set([]byte(id), bts)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

// Read retrieves Session from store.
// Takes session ID.
// If session not found returns ErrSessionNoRecord error.
func (s *FileStore) Read(id string) (ses *Session, err error) {
	err = s.shelf.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(id))
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		ses = new(Session)
		if err := decGob(val, ses); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if err == badger.ErrKeyNotFound || err == badger.ErrEmptyKey {
			err = ErrSessionNoRecord
		}
	}
	return
}

// Update runs a function on Session.
// Takes session ID and a function with Session as parameter.
// If session not found returns ErrSessionNoRecord error.
func (s *FileStore) Update(id string, fn func(*Session)) (err error) {
	err = s.shelf.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(id))
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		ses := new(Session)
		if err := decGob(val, ses); err != nil {
			return err
		}
		fn(ses)
		bts, err := encGob(ses)
		if err != nil {
			return err
		}
		err = txn.Set([]byte(id), bts)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

// Delete removes Session from the store.
// Takes session ID.
func (s *FileStore) Delete(id string) (err error) {
	err = s.shelf.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(id))
		if err != nil {
			return err
		}
		return nil
	})
	return
}

// Encode types to bytes
func encGob(val interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode types from bytes
func decGob(bts []byte, res interface{}) error {
	buf := bytes.NewBuffer(bts)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(res)
	if err != nil {
		return err
	}
	return nil
}

// Vacuum runs GC every nth minutes.
// Takes interval in minutes as int.
func (s *FileStore) vacuum(d int) {
	if d == 0 {
		return
	}
	ticker := time.NewTicker(time.Minute * time.Duration(d))
	run := func() {
	repeat:
		err := s.shelf.RunValueLogGC(0.5)
		if err == nil {
			goto repeat
		}
	}
	for range ticker.C {
		run()
	}
}

// Expire runs a sweep every tic period.
// Removes expired records.
// Takes interval duration. If 0 supplied, defaults to every 60 minutes.
func (s *FileStore) expire(tic time.Duration) {
	if tic == 0 {
		tic = time.Minute * time.Duration(60)
	}
	ticker := time.NewTicker(tic)
	for range ticker.C {
		err := s.shelf.Update(func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.Key()
				val, err := item.ValueCopy(nil)
				if err != nil {
					return err
				}
				ses := new(Session)
				if err := decGob(val, ses); err != nil {
					return err
				}
				if time.Now().After(ses.Expiry) {
					err = txn.Delete(key)
					if err != nil {
						return err
					}
				}
			}
			it.Close()
			return nil
		})
		if err != nil {
			log.Println("expire: ", err)
		}
	}
}
