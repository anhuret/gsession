// Copyright (c), Ruslan Sendecky. All rights reserved.
// Use of this source code is governed by the MIT license.
// See the LICENSE file in the project root for more information.

package gsession

import (
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
	n := "gsession"
	m := New(nil, 0, 0)
	l := "ruslan"

	h := func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/":
			w.Write([]byte(b))
		case "/set":
			err := m.Set(r, k, v)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		case "/get":
			val, err := m.Get(r, k)
			if err != nil {
				http.Error(w, err.Error(), 500)
				break
			}
			w.Write([]byte(val.(string)))
		case "/delete":
			err := m.Delete(r, k)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		case "/remove":
			err := m.Remove(w, r)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		case "/stoken":
			err := m.SetToken(r, l)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		case "/gtoken":
			tok, err := m.Token(r)
			if err != nil {
				http.Error(w, err.Error(), 500)
				break
			}
			w.Write([]byte(tok))
		}
	}

	s := httptest.NewServer(m.Use(http.HandlerFunc(h)))
	defer s.Close()

	t.Run("session crud", func(t *testing.T) {
		e := httpexpect.New(t, s.URL)

		r := e.GET("/").Expect().Status(http.StatusOK)

		r.Cookies().NotEmpty()
		c := r.Cookie(n)
		i := c.Value().Raw()

		e.GET("/set").WithCookie(n, i).Expect().Status(http.StatusOK)

		r = e.GET("/get").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Body().Equal(v)

		e.GET("/stoken").WithCookie(n, i).Expect().Status(http.StatusOK)
		r = e.GET("/gtoken").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Body().Equal(l)

		e.GET("/delete").WithCookie(n, i).Expect().Status(http.StatusOK)
		e.GET("/get").WithCookie(n, i).Expect().Status(http.StatusInternalServerError)
		r = e.GET("/remove").WithCookie(n, i).Expect().Status(http.StatusOK)
		r.Cookie(n).Value().NotEqual(i)
		// Check setting with an old id

	})

}
