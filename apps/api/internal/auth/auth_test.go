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
			_ = json.NewEncoder(w).Encode(map[string]string{"sub": "user-42", "org_id": "org-demo-a"})
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
	if u.OrgID != "org-demo-a" {
		t.Fatalf("org_id %q", u.OrgID)
	}
}

func TestAuthenticateUserInfoRequiresOrgID(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/userinfo" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"sub": "user-42"})
	}))
	defer idp.Close()

	mw := New(false, idp.URL, idp.URL, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer opaque-token")
	if _, err := mw.authenticate(req); err == nil {
		t.Fatal("expected error when org_id missing")
	}
}

func TestAuthenticateDevUserOrgHeader(t *testing.T) {
	mw := New(true, "", "", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Dev-User-Sub", "demo-user-a")
	req.Header.Set("X-Dev-User-Org", "org-demo-b")
	u, err := mw.authenticate(req)
	if err != nil {
		t.Fatal(err)
	}
	if u.Sub != "demo-user-a" || u.OrgID != "org-demo-b" {
		t.Fatalf("got %+v", u)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Dev-User-Sub", "demo-user-a")
	u2, err := mw.authenticate(req2)
	if err != nil {
		t.Fatal(err)
	}
	if u2.OrgID != "org-demo-a" {
		t.Fatalf("default org %q", u2.OrgID)
	}
}

func TestAuthenticateRejectsMissingBearerWhenDevAuthOff(t *testing.T) {
	mw := New(false, "http://issuer.example", "http://issuer.example", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := mw.authenticate(req); err == nil {
		t.Fatal("expected error")
	}
}
