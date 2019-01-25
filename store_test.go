// Copyright (c), Ruslan Sendecky. All rights reserved
// Use of this source code is governed by the MIT license
// See the LICENSE file in the project root for more information

package gsession

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func TestStore(t *testing.T) {
	var ms *MemoryStore
	var fs *FileStore
	testExpiry := func(store Store) error {
		id := uuid.New().String()
		err := store.Create(id, nil)
		if err != nil {
			return err
		}
		err = store.Update(id, func(ses *Session) {
			ses.Origin = time.Now().AddDate(0, 0, -3)
		})
		if err != nil {
			return err
		}
		err = store.Expire(time.Hour * 24)
		if err != nil {
			return err
		}
		_, err = store.Read(id)
		if err == nil {
			errors.New("read should return error")
		}
		if err != ErrSessionNoRecord {
			errors.Wrap(err, "should return ErrSessionNoRecord")
		}
		return nil
	}

	testStore := func(store Store) error {
		id := uuid.New().String()
		key := uuid.New().String()
		value := uuid.New().String()
		var ses *Session
		err := store.Create(id, &Session{})
		if err != nil {
			return err
		}
		err = store.Create(id, nil)
		if err != nil {
			return err
		}
		ses, err = store.Read(id)
		if err != nil {
			return err
		}
		err = store.Update(id, func(s *Session) {
			s.Token = value
		})
		if err != nil {
			return err
		}
		err = store.Update(id, func(s *Session) {
			s.Data[key] = value
		})
		if err != nil {
			return err
		}
		ses, err = store.Read(id)
		if err != nil {
			return err
		}
		v := ses.Data[key]
		if v != value {
			return errors.New("invalid store data returned")
		}
		err = store.Update(id, func(s *Session) {
			delete(s.Data, key)
		})
		if err != nil {
			return err
		}
		err = store.Delete(id)
		if err != nil {
			return err
		}
		return nil
	}

	runBatch := func(store Store) error {
		var wg sync.WaitGroup
		rounds := 100
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
				err := testStore(store)
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
	t.Run("memory store", func(t *testing.T) {
		ms = NewMemoryStore()
		if ms == nil {
			t.Fatal("memory store error")
		}
		err := runBatch(ms)
		if err != nil {
			t.Fatal(err)
		}
		err = testExpiry(NewMemoryStore())
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("file store", func(t *testing.T) {
		fs = NewFileStore("")
		if fs == nil {
			t.Fatal("file store error")
		}
		err := runBatch(fs)
		if err != nil {
			t.Fatal(err)
		}
		os.RemoveAll("session")
		err = testExpiry(NewFileStore(""))
		if err != nil {
			t.Fatal(err)
		}
		os.RemoveAll("session")
	})

}
