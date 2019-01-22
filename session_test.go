// Copyright (c), Ruslan Sendecky. All rights reserved.
// Use of this source code is governed by the MIT license.
// See the LICENSE file in the project root for more information.

package gsession

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
			s, err := man.Get(r, p["key"])
			if err != nil {
				if err == ErrSessionKeyInvalid {
					http.Error(w, err.Error(), http.StatusNoContent)
					break
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				break
			}
			w.Write([]byte(s.(string)))
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
		}
	}

	sessionTest := func(t *testing.T, u string) {
		e := httpexpect.New(t, u)

		// New session created. New ID is issued.
		i := e.GET("/").Expect().Status(http.StatusOK).Cookie(n).Value().NotEmpty().Raw()

		// Correct current ID is sent back. No new ID is issued and no cookie set.
		e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK).Cookies().Empty()

		// Wrong ID is sent. Session is invalidated. New ID is re-issued and new coockie is set.
		r := e.GET("/").WithCookie(n, w).Expect().Status(http.StatusOK)
		i = r.Cookie(n).Value().NotEmpty().NotEqual(i).NotEqual(w).Raw()

		// New ID is sent back.
		e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK).Cookies().Empty()

		// Get session data with a key that does not exist
		r = e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusNoContent)
		r.Body().Empty()

		// Set session data
		e.PUT("/set").WithCookie(n, i).WithJSON(val).Expect().Status(http.StatusOK)

		// Get back session data
		r = e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusOK)
		r.Body().NotEmpty().Equal(v)

		// Delete session key
		e.PUT("/delete").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusOK)

		// Get deleted key
		r = e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusNoContent)
		r.Body().Empty()

		// Set session token
		e.GET("/stoken").WithCookie(n, i).Expect().Status(http.StatusOK)

		// Get session token
		e.GET("/gtoken").WithCookie(n, i).Expect().Status(http.StatusOK).Body().NotEmpty().Equal(l)

		// Set session data
		e.PUT("/set").WithCookie(n, i).WithJSON(val).Expect().Status(http.StatusOK)

		// Move expiry back in time
		err := man.store.Update(i, func(ses *Session) {
			ses.Expiry = time.Now().AddDate(0, 0, -3)
		})
		if err != nil {
			t.Fatal(err)
		}

		// Trigger expiry and regenerate ID. Session data is cleared
		r = e.GET("/").WithCookie(n, i).Expect().Status(http.StatusOK)
		i = r.Cookie(n).Value().NotEmpty().NotEqual(i).Raw()
		r = e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusNoContent)
		r.Body().Empty()

		// Set new token and session data
		e.GET("/stoken").WithCookie(n, i).Expect().Status(http.StatusOK)
		e.PUT("/set").WithCookie(n, i).WithJSON(val).Expect().Status(http.StatusOK)

		// Move idle back in time
		err = man.store.Update(i, func(ses *Session) {
			ses.Tstamp = time.Now().Add(-2 * time.Hour)
		})
		if err != nil {
			t.Fatal(err)
		}

		// New ID generated. Token is cleard. Sesion data retained.
		r = e.GET("/gtoken").WithCookie(n, i).Expect().Status(http.StatusOK)
		i = r.Cookie(n).Value().NotEmpty().NotEqual(i).Raw()
		r.Body().Empty()
		r = e.PUT("/get").WithCookie(n, i).WithJSON(key).Expect().Status(http.StatusOK)
		r.Body().NotEmpty().Equal(v)

		// Remove session record
		r = e.GET("/remove").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookies().NotEmpty()
		r.Cookie(n).Value().NotEmpty().NotEqual(i)
	}

	t.Run("memory store", func(t *testing.T) {
		man = New(NewMemoryStore(0), 0, 0)
		s := httptest.NewServer(man.Use(http.HandlerFunc(handler)))
		defer s.Close()
		sessionTest(t, s.URL)
	})

	t.Run("file store", func(t *testing.T) {
		man = New(NewFileStore("", 0), 0, 0)
		s := httptest.NewServer(man.Use(http.HandlerFunc(handler)))
		defer s.Close()
		sessionTest(t, s.URL)
		os.RemoveAll("session")
	})
}
