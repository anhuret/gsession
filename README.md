# gsession

Very simple Go session management library. Provides memory and filesystem stores.

## Features

* Absolute expiry, regardless of session activity
* Idle inactivity timeout
* Renewal timeout when session ID is renewed regardless of the session activity or idle timeout
* Expiry, idle and renew timeouts can be disabled
* Uses excellent Badger KV DB for on disk persistance

## Install

```
go get -u github.com/anhuret/gsession

```

## Go version

Built and tested with Go 1.14

## Usage

```go

package main

import (
	gs "github.com/anhuret/gsession"
	"net/http"
)

func main() {
	// Create default session manager
	// Use nil to get default memory store
	// Use 0 for expiry, idle and renew to get defaults: 24H, 1H and 30M respectively
	manager := gs.New(nil, 0, 0, 0)

	// Handler function
	sayHello := func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/set":
			err := manager.Set(r, "key", "Hello, gsession\n")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/get":
			val, err := manager.Get(r, "key")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				break
			}
			w.Write([]byte(val.(string)))
		}
	}

	// Wrap your handler function in gsession middlware
	handler := manager.Use(http.HandlerFunc(sayHello))

	http.ListenAndServe(":8080", handler)
}

```

```
[ruslan@weasel ~]$ curl -I -c cookie http://localhost:8080/set
HTTP/1.1 200 OK
Set-Cookie: gsession=e7735c38-0368-475a-852f-290774f2679f; Path=/; Expires=Thu, 24 Jan 2019 11:43:52 GMT; HttpOnly
Date: Wed, 23 Jan 2019 11:43:52 GMT

[ruslan@weasel ~]$ curl -b cookie http://localhost:8080/get
Hello, gsession

```

To use persistent session store with Badger backend

```go
// Give it a directory or leave blank to get default "gsession"
manager := gs.New(gs.NewFileStore("some_directory", 0), 0, 0, 0)
```

## Test

Run go test from the project root

```
go test -cover

PASS
coverage: 83.7% of statements
ok  	github.com/anhuret/gsession	0.382s

```

## Why?

This is my humble attempt at learning Go. A session manager seemed like a good choice.\

## License

[MIT](LICENSE)
