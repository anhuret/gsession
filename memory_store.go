// Copyright (c), Ruslan Sendecky. All rights reserved.
// Use of this source code is governed by the MIT license.
// See the LICENSE file in the project root for more information.

package gsession

import (
	"sync"
	"time"
)

// MemoryStore struct.
type MemoryStore struct {
	sync.RWMutex
	shelf map[string]*Session
}

// NewMemoryStore creates a new memory store.
// Takes ticker period.
// Ticker sets duration for how often expired sessions are cleaned up.
func NewMemoryStore(tic time.Duration) *MemoryStore {
	store := &MemoryStore{
		shelf: make(map[string]*Session),
	}
	go store.expire(tic)
	return store
}

// Create adds a new session entry to the store.
// Takes a session ID and session expiry duration.
func (s *MemoryStore) Create(id string, exp time.Duration) error {
	s.Lock()
	defer s.Unlock()
	s.shelf[id] = &Session{
		Expiry: time.Now().Add(exp),
		Tstamp: time.Now(),
		Token:  "",
		Data:   make(map[string]interface{}),
	}
	return nil
}

// Read retrieves Session from store.
// Takes session ID.
// If session not found returns ErrSessionNoRecord error.
func (s *MemoryStore) Read(id string) (*Session, error) {
	s.RLock()
	defer s.RUnlock()
	if ses, ok := s.shelf[id]; ok {
		scp := *ses
		return &scp, nil
	}
	return nil, ErrSessionNoRecord
}

// Update runs a function on Session.
// Takes session ID and a function with Session as parameter.
// If session not found returns ErrSessionNoRecord error.
func (s *MemoryStore) Update(id string, fn func(*Session)) (err error) {
	s.Lock()
	defer s.Unlock()
	if ses, ok := s.shelf[id]; ok {
		fn(ses)
		return nil
	}
	return ErrSessionNoRecord
}

// Delete removes Session from the store.
// Takes session ID.
func (s *MemoryStore) Delete(id string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.shelf, id)
	return nil
}

// Expire runs a sweep every tic period.
// Removes expired records.
// Takes interval duration. If 0 supplied, defaults to every 60 minutes.
func (s *MemoryStore) expire(tic time.Duration) {
	run := func() {
		s.Lock()
		for key, ses := range s.shelf {
			if time.Now().After(ses.Expiry) {
				delete(s.shelf, key)
			}
		}
		s.Unlock()
	}
	if tic == 0 {
		tic = time.Minute * time.Duration(60)
	}
	ticker := time.NewTicker(tic)
	for range ticker.C {
		run()
	}
}
