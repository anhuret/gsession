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
	m := New(nil, 0, 0)

	h := func(w http.ResponseWriter, r *http.Request) {
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
			err := m.Set(r, p["key"], p["val"])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/get":
			val, err := m.Get(r, p["key"])
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
			err := m.Delete(r, p["key"])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/remove":
			err := m.Remove(w, r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/stoken":
			err := m.SetToken(r, l)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/gtoken":
			tok, err := m.Token(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				break
			}
			w.Write([]byte(tok))
		case "/reset":
			err := m.Reset(w, r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	s := httptest.NewServer(m.Use(http.HandlerFunc(h)))
	defer s.Close()

	t.Run("session crud", func(t *testing.T) {
		e := httpexpect.New(t, s.URL)

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
	})

}
