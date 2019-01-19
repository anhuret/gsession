// Copyright (c), Ruslan Sendecky. All rights reserved.
// Use of this source code is governed by the MIT license.
// See the LICENSE file in the project root for more information.

package gsession

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gavv/httpexpect"
	"github.com/google/uuid"
)

func TestSession(t *testing.T) {
	b := uuid.New().String()
	k := uuid.New().String()
	v := uuid.New().String()
	w := uuid.New().String()
	n := "gsession"
	l := "ruslan"
	key := map[string]string{
		"key": k,
	}
	val := map[string]string{
		"key": k,
		"val": v,
	}

	var man *Manager

	handler := func(w http.ResponseWriter, r *http.Request) {
		var p map[string]string
		if r.Body != nil {
			err := json.NewDecoder(r.Body).Decode(&p)
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		switch r.RequestURI {
		case "/":
			w.Write([]byte(b))
		case "/set":
			err := man.Set(r, p["key"], p["val"])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/get":
			val, err := man.Get(r, p["key"])
			if err != nil {
				if err == ErrSessionKeyInvalid {
					http.Error(w, err.Error(), http.StatusNoContent)
					break
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				break
			}
			w.Write([]byte(val.(string)))
		case "/delete":
			err := man.Delete(r, p["key"])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/remove":
			err := man.Remove(w, r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/stoken":
			_, err := man.Token(r, &l)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/gtoken":
			tok, err := man.Token(r, nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				break
			}
			w.Write([]byte(tok))
		case "/reset":
			err := man.Reset(w, r, false)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	sessionCrud := func(t *testing.T, u string) {

		e := httpexpect.New(t, u)

		// New session created. New ID is issued.
		r := e.GET("/").Expect().Status(http.StatusOK)
		r.Cookies().NotEmpty()
		i := r.Cookie(n).Value().NotEmpty().Raw()

		// Correct current ID is sent back. No new ID is issued and no cookie set.
		r = e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().Empty()

		// Wrong ID is sent. Session is invalidated. New ID is re-issued and new coockie is set.
		r = e.GET("/").WithCookie(n, w).Expect().Status(http.StatusOK)
		r.Cookies().NotEmpty()
		i = r.Cookie(n).Value().NotEmpty().NotEqual(i).NotEqual(w).Raw()

		// New ID is sent back. No new ID is issued and no cookie set.
		r = e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().Empty()

		// Get session data with a key that does not exist
		e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusNoContent)

		// Set session data
		e.PUT("/set").WithCookie(n, i).WithJSON(val).Expect().Status(http.StatusOK)

		// Get back session data
		r = e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusOK)
		r.Cookies().Empty()
		r.Body().NotEmpty().Equal(v)

		// Reset session and generate new ID
		r = e.GET("/reset").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().NotEmpty()
		i = r.Cookie(n).Value().NotEmpty().NotEqual(i).Raw()

		// Get back session data after reset
		r = e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusOK)
		r.Cookies().Empty()
		r.Body().NotEmpty().Equal(v)

		// Delete session key
		e.PUT("/delete").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusOK)

		// Get deleted key
		e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusNoContent)

		// Set session token
		e.GET("/stoken").WithCookie(n, i).Expect().Status(http.StatusOK)

		// Get session token
		r = e.GET("/gtoken").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Body().NotEmpty().Equal(l)

		// Remove session record
		r = e.GET("/remove").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().NotEmpty()
		r.Cookie(n).Value().NotEmpty().NotEqual(i)

	}

	t.Run("memory session crud", func(t *testing.T) {
		man = New(NewMemoryStore(0), 0, 0)
		s := httptest.NewServer(man.Use(http.HandlerFunc(handler)))
		defer s.Close()
		sessionCrud(t, s.URL)
	})
	/*
		t.Run("file session crud", func(t *testing.T) {
			man = New(NewFileStore("", 0), 0, 0)
			s := httptest.NewServer(man.Use(http.HandlerFunc(handler)))
			defer s.Close()
			sessionCrud(t, s.URL)
			os.RemoveAll("session")
		})
	*/
	t.Run("memory session expiry", func(t *testing.T) {
		man = New(nil, 0, 0)
		s := httptest.NewServer(man.Use(http.HandlerFunc(handler)))
		defer s.Close()

		e := httpexpect.New(t, s.URL)

		r := e.GET("/").Expect().Status(http.StatusOK)
		r.Cookies().NotEmpty()
		i := r.Cookie(n).Value().NotEmpty().Raw()

		r = e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().Empty()

		err := man.store.Update(i, func(ses *Session) {
			ses.Expiry = time.Now().AddDate(0, 0, -3)
		})
		if err != nil {
			t.Fatal(err)
		}

		r = e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().NotEmpty()
		i = r.Cookie(n).Value().NotEmpty().NotEqual(i).Raw()

		r = e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().Empty()

		e.GET("/stoken").WithCookie(n, i).Expect().Status(http.StatusOK)

		r = e.GET("/gtoken").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Body().NotEmpty().Equal(l)

		err = man.store.Update(i, func(ses *Session) {
			ses.Tstamp = time.Now().Add(-2 * time.Hour)
		})
		if err != nil {
			t.Fatal(err)
		}

		r = e.GET("/gtoken").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Body().Empty().NotEqual(l)

	})

}
