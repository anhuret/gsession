// Copyright (c), Ruslan Sendecky. All rights reserved.
// Use of this source code is governed by the MIT license.
// See the LICENSE file in the project root for more information.

package gsession

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Manager type
type Manager struct {
	name   string
	expiry time.Duration
	idle   time.Duration
	store  Store
}

// Store interface
type Store interface {
	Create(string, time.Duration) error
	Read(string) (*Session, error)
	Update(string, func(*Session)) error
	Delete(string) error
}

// Session struct stores session data
type Session struct {
	Expiry time.Time
	Tstamp time.Time
	Token  string
	Data   map[string]interface{}
}

var (
	// ErrSessionNilContext  - request session context is nil
	ErrSessionNilContext = errors.New("request session context is nil")
	// ErrSessionKeyInvalid - session data key does not exist or invalid
	ErrSessionKeyInvalid = errors.New("session data key does not exist or invalid")
	// ErrSessionNoRecord - session record does not exist or invalid
	ErrSessionNoRecord = errors.New("session record does not exist or invalid")
)

// Context key type
type ctxkey int

// Context key constant
const sesID ctxkey = 0

// Session validation type
type sesval int

// Session validation constants
const (
	sesError sesval = iota
	sesExpired
	sesInvalid
	sesIdle
	sesPass
)

//func init() { log.SetFlags(log.Lshortfile | log.LstdFlags) }

// New returns new session manager
func New(store Store, expiry, idle time.Duration) *Manager {
	if expiry == 0 {
		expiry = time.Minute * time.Duration(1440)
	}
	if idle == 0 {
		idle = time.Minute * time.Duration(60)
	}
	if store == nil {
		store = NewMemoryStore(0)
	}
	return &Manager{
		name:   "gsession",
		expiry: expiry,
		idle:   idle,
		store:  store,
	}
}

// Use provides middleware session handler
func (m *Manager) Use(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := m.register(w, r)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		ctx := context.WithValue(r.Context(), sesID, id)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Register validates and registers new session record.
func (m *Manager) register(w http.ResponseWriter, r *http.Request) (string, error) {
	var id string
	jar, err := r.Cookie(m.name)
	if err == nil && jar.Value != "" {
		id = jar.Value
		val, err := m.validate(id)
		if err != nil {
			return "", err
		}
		if val == sesPass {
			err = m.store.Update(id, func(ses *Session) {
				ses.Tstamp = time.Now()
			})
			if err != nil {
				return "", err
			}
			return id, nil
		}
		if val == sesIdle {
			id, err = m.reset(w, r, id, true)
			if err != nil {
				return "", err
			}
			m.putCookie(w, id)
			return id, nil
		}
	}
	id = uuid.New().String()
	err = m.store.Create(id, m.expiry)
	if err != nil {
		return "", err
	}
	m.putCookie(w, id)
	return id, nil
}

// Validate checks session record, expiry and idle time.
func (m *Manager) validate(id string) (sesval, error) {
	ses, err := m.store.Read(id)
	if err != nil {
		if err == ErrSessionNoRecord {
			return sesInvalid, nil
		}
		return sesError, err
	}
	if time.Now().After(ses.Expiry) {
		return sesExpired, nil
	}
	if time.Now().After(ses.Tstamp.Add(m.idle)) {
		return sesIdle, nil
	}
	return sesPass, nil
}

// Set sets new session data.
// Takes HTTP request, key and value.
func (m *Manager) Set(r *http.Request, k string, v string) error {
	id, err := sesCtx(r)
	if err != nil {
		return err
	}
	err = m.store.Update(id, func(ses *Session) {
		ses.Data[k] = v
	})
	return err
}

// Get returns session data.
// Takes HTTP request and data key.
func (m *Manager) Get(r *http.Request, k string) (interface{}, error) {
	id, err := sesCtx(r)
	if err != nil {
		return nil, err
	}
	ses, err := m.store.Read(id)
	if err != nil {
		return nil, err
	}
	if dat, ok := ses.Data[k]; ok {
		return dat, nil
	}
	return nil, ErrSessionKeyInvalid
}

// Delete removes session data.
// Takes HTTP request and key.
func (m *Manager) Delete(r *http.Request, k string) error {
	id, err := sesCtx(r)
	if err != nil {
		return err
	}
	err = m.store.Update(id, func(ses *Session) {
		delete(ses.Data, k)
	})
	return err
}

// Token sets or gets session token.
// Takes HTTP request and a token string pointer.
// Returns current token or error.
// Pass nil to get the current token.
// Pass string pointer to set a new token.
func (m *Manager) Token(r *http.Request, t *string) (string, error) {
	id, err := sesCtx(r)
	if err != nil {
		return "", err
	}
	if t == nil {
		ses, err := m.store.Read(id)
		if err != nil {
			return "", err
		}
		return ses.Token, nil
	}
	err = m.store.Update(id, func(ses *Session) {
		ses.Token = *t
	})
	if err != nil {
		return "", err
	}
	return *t, nil
}

// Remove deletes existing session record. Generates new session ID.
// Takes HTTP request and response.
func (m *Manager) Remove(w http.ResponseWriter, r *http.Request) error {
	id, err := sesCtx(r)
	if err != nil {
		return err
	}
	err = m.store.Delete(id)
	if err != nil {
		return err
	}
	id = uuid.New().String()
	err = m.store.Create(id, m.expiry)
	if err != nil {
		return err
	}
	m.putCookie(w, id)
	return nil
}

// Reset generates new session ID. Keeps old session data if false is given for token reset.
// Set third parameter to true to get Token reset and re-touch Tstamp
func (m *Manager) reset(w http.ResponseWriter, r *http.Request, id string, t bool) (string, error) {
	osd, err := m.store.Read(id)
	if err != nil {
		return "", err
	}
	id = uuid.New().String()
	err = m.store.Create(id, m.expiry)
	if err != nil {
		return "", err
	}
	if t {
		osd.Token = ""
		osd.Tstamp = time.Now()
	}
	err = m.store.Update(id, func(ses *Session) {
		*ses = *osd
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// Put cookie in response.
func (m *Manager) putCookie(w http.ResponseWriter, id string) {
	exp := time.Now().Add(m.expiry)
	jar := http.Cookie{Name: m.name, Value: id, Expires: exp, Path: "/"}
	http.SetCookie(w, &jar)
}

// Returns session ID from request context.
func sesCtx(r *http.Request) (string, error) {
	ctx := r.Context().Value(sesID)
	if ctx == nil {
		return "", ErrSessionNilContext
	}
	return ctx.(string), nil
}
