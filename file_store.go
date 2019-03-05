// Copyright (c), Ruslan Sendecky. All rights reserved
// Use of this source code is governed by the MIT license
// See the LICENSE file in the project root for more information

package gsession

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"

	"github.com/dgraph-io/badger"
)

// FileStore struct
type FileStore struct {
	shelf *badger.DB
}

// NewFileStore creates a new file store
// Takes directory path for the database files
// Empty directory string defaults to "session"
func NewFileStore(dir string) *FileStore {
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
	go store.vacuum(time.Hour * 12)
	return store
}

// Create adds a new session entry to the store
// Takes a session ID and Session struct or nil
// Pass nil to create default session
// Psss Session pointer to create an entry with pre defined data or overwrite existing
func (s *FileStore) Create(id string, ses *Session) (err error) {
	if ses == nil {
		ses = &Session{
			Origin: time.Now(),
			Tstamp: time.Now(),
			Token:  "",
			Data:   make(map[string]interface{}),
		}
	} else {
		if ses.Origin.IsZero() {
			ses.Origin = time.Now()
		}
		if ses.Tstamp.IsZero() {
			ses.Tstamp = time.Now()
		}
		if ses.Data == nil {
			ses.Data = make(map[string]interface{})
		}
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

// Read retrieves Session from store
// Takes session ID
// If session not found returns ErrSessionNoRecord error
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

// Update runs a function on Session
// Takes session ID and a function with Session as parameter
// If session not found returns ErrSessionNoRecord error
func (s *FileStore) Update(id string, run func(*Session)) (err error) {
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
		run(ses)
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

// Delete removes Session from the store
// Takes session ID
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

// Expire removes expired records
// Takes expiration duration
func (s *FileStore) Expire(exp time.Duration) (err error) {
	err = s.shelf.Update(func(txn *badger.Txn) error {
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
			if time.Now().After(ses.Origin.Add(exp)) {
				err = txn.Delete(key)
				if err != nil {
					return err
				}
			}
		}
		it.Close()
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

// Vacuum runs GC every nth
// Takes interval as duration
func (s *FileStore) vacuum(d time.Duration) {
	if d == 0 {
		return
	}
	ticker := time.NewTicker(d)
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
