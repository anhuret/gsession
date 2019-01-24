// Copyright (c), Ruslan Sendecky. All rights reserved
// Use of this source code is governed by the MIT license
// See the LICENSE file in the project root for more information

package gsession

import (
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func TestStore(t *testing.T) {
	var ms *MemoryStore
	var fs *FileStore
	t.Run("memory store", func(t *testing.T) {
		ms = NewMemoryStore()
		if ms == nil {
			t.Fatal("memory store error")
		}
		err := testStore(ms)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("file store", func(t *testing.T) {
		fs = NewFileStore("")
		if fs == nil {
			t.Fatal("file store error")
		}
		err := testStore(fs)
		if err != nil {
			t.Fatal(err)
		}
	})
	os.RemoveAll("session")
}

func testStore(store Store) error {
	var wg sync.WaitGroup
	rounds := 1000
	wg.Add(rounds)
	erc := make(chan error, 1)
	done := make(chan bool, 1)
	go func() {
		wg.Wait()
		close(done)
	}()
	for i := 1; i < rounds+1; i++ {
		go func() {
			defer wg.Done()
			err := storeCrud(store)
			if err != nil {
				erc <- err
			}
		}()
	}
	select {
	case <-done:
		return nil
	case err := <-erc:
		return err
	}
}

func storeCrud(store Store) error {
	id := uuid.New().String()
	key := uuid.New().String()
	value := uuid.New().String()
	var err error
	var ses *Session

	err = store.Create(id)
	if err != nil {
		return errors.Wrap(err, "create session record")
	}

	ses, err = store.Read(id)
	if err != nil {
		return errors.Wrap(err, "read session record")
	}

	err = store.Update(id, func(s *Session) {
		s.Token = value
	})
	if err != nil {
		return errors.Wrap(err, "update session record")
	}

	err = store.Update(id, func(s *Session) {
		s.Data[key] = value
	})
	if err != nil {
		return errors.Wrap(err, "set session data")
	}

	ses, err = store.Read(id)
	if err != nil {
		return errors.Wrap(err, "get session data")
	}
	v := ses.Data[key]
	if v != value {
		return errors.Wrap(err, "session data does not match")
	}

	err = store.Update(id, func(s *Session) {
		delete(s.Data, key)
	})
	if err != nil {
		return errors.Wrap(err, "delete session data")
	}

	err = store.Delete(id)
	if err != nil {
		return errors.Wrap(err, "delete session record")
	}
	return nil
}
