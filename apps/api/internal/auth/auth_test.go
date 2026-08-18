package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticateOpaqueAccessTokenViaUserInfo(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer opaque-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"sub": "user-42"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer idp.Close()

	mw := New(false, idp.URL, idp.URL, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer opaque-token")
	u, err := mw.authenticate(req)
	if err != nil {
		t.Fatal(err)
	}
	if u.Sub != "user-42" {
		t.Fatalf("sub %q", u.Sub)
	}
}

func TestAuthenticateRejectsMissingBearerWhenDevAuthOff(t *testing.T) {
	mw := New(false, "http://issuer.example", "http://issuer.example", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := mw.authenticate(req); err == nil {
		t.Fatal("expected error")
	}
}
