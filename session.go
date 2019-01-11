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
		if val == sesPass || val == sesIdle {
			err = m.store.Update(id, func(ses *Session) {
				ses.Tstamp = time.Now()
				if val == sesIdle {
					ses.Token = ""
				}
			})
			if err != nil {
				return "", err
			}
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

// SetToken sets session token.
// Takes HTTP request and a token string.
func (m *Manager) SetToken(r *http.Request, t string) error {
	id, err := sesctx(r)
	if err != nil {
		return err
	}
	err = m.store.Update(id, func(ses *Session) {
		ses.Token = t
	})
	return err
}

// Token gets session token.
// Takes HTTP request.
func (m *Manager) Token(r *http.Request) (string, error) {
	id, err := sesctx(r)
	if err != nil {
		return "", err
	}
	ses, err := m.store.Read(id)
	if err != nil {
		return "", err
	}
	return ses.Token, nil
}

// Reset updates session absolute expiry time stamp.
func (m *Manager) Reset(w http.ResponseWriter, r *http.Request) error {
	id, err := sesctx(r)
	if err != nil {
		return err
	}
	err = m.store.Update(id, func(ses *Session) {
		ses.Expiry = time.Now().Add(m.expiry)
	})
	if err != nil {
		return err
	}
	m.putCookie(w, id)
	return nil
}

// Get returns session data.
// Takes HTTP request and data key.
func (m *Manager) Get(r *http.Request, k string) (interface{}, error) {
	id, err := sesctx(r)
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

// Set sets new session data.
// Takes HTTP request, key and value.
func (m *Manager) Set(r *http.Request, k string, v string) error {
	id, err := sesctx(r)
	if err != nil {
		return err
	}
	err = m.store.Update(id, func(ses *Session) {
		ses.Data[k] = v
	})
	return err
}

// Delete removes session data.
// Takes HTTP request and key.
func (m *Manager) Delete(r *http.Request, k string) error {
	id, err := sesctx(r)
	if err != nil {
		return err
	}
	err = m.store.Update(id, func(ses *Session) {
		delete(ses.Data, k)
	})
	return err
}

// Remove removes existing session record.
// Takes HTTP request and response.
func (m *Manager) Remove(w http.ResponseWriter, r *http.Request) error {
	id, err := sesctx(r)
	if err != nil {
		return err
	}
	err = m.store.Delete(id)
	if err != nil {
		return err
	}
	m.remCookie(w)
	return nil
}

// Put cookie in response.
func (m *Manager) putCookie(w http.ResponseWriter, id string) {
	exp := time.Now().Add(m.expiry)
	jar := http.Cookie{Name: m.name, Value: id, Expires: exp, Path: "/"}
	http.SetCookie(w, &jar)
}

// Remove cookie by invalidating it.
func (m *Manager) remCookie(w http.ResponseWriter) {
	exp := time.Now()
	jar := http.Cookie{Name: m.name, Value: "", Expires: exp}
	http.SetCookie(w, &jar)
}

// Returns session ID from request context.
func sesctx(r *http.Request) (string, error) {
	ctx := r.Context().Value(sesID)
	if ctx == nil {
		return "", ErrSessionNilContext
	}
	return ctx.(string), nil
}
