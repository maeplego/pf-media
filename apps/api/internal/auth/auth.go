package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type ctxKey struct{}

type User struct {
	Sub string
}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

type Middleware struct {
	devAuth    bool
	issuer     string
	audience   string
	jwks       jwk.Set
	jwksMu     sync.RWMutex
	jwksLoaded time.Time
}

func New(devAuth bool, issuer, audience string) *Middleware {
	return &Middleware{devAuth: devAuth, issuer: issuer, audience: audience}
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := m.authenticate(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
	})
}

func (m *Middleware) authenticate(r *http.Request) (User, error) {
	if h := strings.TrimSpace(r.Header.Get("X-Dev-User-Sub")); h != "" && m.devAuth {
		return User{Sub: h}, nil
	}
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authz, "Bearer ") {
		return User{}, fmt.Errorf("missing bearer")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	if token == "" {
		return User{}, fmt.Errorf("missing bearer")
	}
	if m.issuer == "" {
		return User{}, fmt.Errorf("oidc not configured")
	}
	set, err := m.jwksSet(r.Context())
	if err != nil {
		return User{}, err
	}
	tok, err := jwt.Parse([]byte(token), jwt.WithKeySet(set), jwt.WithIssuer(m.issuer))
	if err != nil {
		return User{}, err
	}
	if m.audience != "" {
		found := false
		for _, aud := range tok.Audience() {
			if aud == m.audience {
				found = true
				break
			}
		}
		if !found {
			return User{}, fmt.Errorf("audience mismatch")
		}
	}
	sub := tok.Subject()
	if sub == "" {
		return User{}, fmt.Errorf("empty sub")
	}
	return User{Sub: sub}, nil
}

func (m *Middleware) jwksSet(ctx context.Context) (jwk.Set, error) {
	m.jwksMu.RLock()
	if m.jwks != nil && time.Since(m.jwksLoaded) < 5*time.Minute {
		defer m.jwksMu.RUnlock()
		return m.jwks, nil
	}
	m.jwksMu.RUnlock()

	m.jwksMu.Lock()
	defer m.jwksMu.Unlock()
	if m.jwks != nil && time.Since(m.jwksLoaded) < 5*time.Minute {
		return m.jwks, nil
	}
	url := m.issuer + "/.well-known/jwks.json"
	set, err := jwk.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	m.jwks = set
	m.jwksLoaded = time.Now()
	return set, nil
}

func ProcessorToken(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			if got == "" || got != expected {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
