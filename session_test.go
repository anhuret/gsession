package gsession

import (
	"net/http"
	"testing"
)

func TestSession(t *testing.T) {
	var man *Manager
	key := "ruslan"
	value := "sendecky"
	t.Run("create session manager", func(t *testing.T) {
		man = New(nil, 0, 0)
		if man == nil {
			t.Fatal("session manager create error")
		}
	})

	t.Run("use manager without context", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/", nil)
		err := man.Set(req, key, value)
		if err == nil {
			t.Fatal("set without context - should return error")
		}
	})

}
