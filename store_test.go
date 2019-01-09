// Copyright (c) 2016, Ruslan Sendecky. All rights reserved.
// Use of this source code is governed by the MIT license.

package gsession

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMemoryStore(t *testing.T) {
	id := uuid.New().String()
	var store *MemoryStore
	t.Run("create memory store", func(t *testing.T) {
		store = NewMemoryStore(10)
		if store == nil {
			t.Fatal("memory store create error")
		}
	})
	testStore(store, id, t)
}

func TestFileStore(t *testing.T) {
	id := uuid.New().String()
	var store *FileStore
	var err error
	t.Run("create file store", func(t *testing.T) {
		store = NewFileStore("", 10)
		if err != nil {
			t.Fatal("file store create error")
		}
	})
	testStore(store, id, t)
	os.RemoveAll("session")
}

func testStore(store Store, id string, t *testing.T) {
	key := "user"
	value := "ruslan"
	var err error
	var ses *Session

	t.Run("create session record", func(t *testing.T) {
		err = store.Create(id, time.Minute*time.Duration(1440))
		if err != nil {
			t.Error("create session record: ", err)
		}
	})

	t.Run("read session record", func(t *testing.T) {
		ses, err = store.Read(id)
		if err != nil {
			t.Error("read session record: ", err)
		}
	})

	t.Run("update session record", func(t *testing.T) {
		err = store.Update(id, func(s *Session) {
			s.Token = value
		})
		if err != nil {
			t.Error("update session record: ", err)
		}
	})

	t.Run("set session data", func(t *testing.T) {
		err = store.Update(id, func(s *Session) {
			s.Data[key] = value
		})
		if err != nil {
			t.Error("set session data: ", err)
		}
	})

	t.Run("get session data", func(t *testing.T) {
		ses, err = store.Read(id)
		if err != nil {
			t.Error("get session data: ", err)
		}
		v := ses.Data[key]
		if v != value {
			t.Error("session data does not match")
		}
	})

	t.Run("delete session data", func(t *testing.T) {
		err = store.Update(id, func(s *Session) {
			delete(s.Data, key)
		})
		if err != nil {
			t.Error("delete session data: ", err)
		}
	})

	t.Run("delete session record", func(t *testing.T) {
		err = store.Delete(id)
		if err != nil {
			t.Error("delete session record: ", err)
		}
	})
}
