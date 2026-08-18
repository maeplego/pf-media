package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/portfolio/pf-media/api/internal/auth"
	"github.com/portfolio/pf-media/api/internal/domain"
	"github.com/portfolio/pf-media/api/internal/service"
	mem "github.com/portfolio/pf-media/api/internal/store/memory"
)

type stubObjects struct{}

func (stubObjects) PresignPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "http://put/" + key, nil
}
func (stubObjects) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "http://get/" + key, nil
}
func (stubObjects) Stat(_ context.Context, _ string) (int64, string, error) { return 10, "etag", nil }
func (stubObjects) Delete(_ context.Context, _ string) error                { return nil }
func (stubObjects) DeletePrefix(_ context.Context, _ string) error          { return nil }
func (stubObjects) Bucket() string                                          { return "media" }

func TestWriteErrTooLarge(t *testing.T) {
	rec := httptest.NewRecorder()
	writeErr(rec, domain.ErrTooLarge)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWriteErrExpired(t *testing.T) {
	rec := httptest.NewRecorder()
	writeErr(rec, domain.ErrExpired)
	if rec.Code != http.StatusGone {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWriteShareErrPasswordRequired(t *testing.T) {
	rec := httptest.NewRecorder()
	writeShareErr(rec, domain.ErrPasswordRequired)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Body.String() == "" || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestPublicShareDoesNotRequireLogin(t *testing.T) {
	store := mem.New()
	svc := service.NewMedia(store, stubObjects{}, nil, 10_000, 5000, time.Minute)
	ctx := context.Background()
	if err := store.CreatePendingFile(ctx, domain.File{
		ID: "f1", OwnerSub: "owner", ObjectKey: "user/owner/f1/orig",
		ContentType: "image/png", SizeBytes: 10, Purpose: "drive",
		Status: domain.FileReady, CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	link, err := svc.CreateShareLink(ctx, "owner", "f1", time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}

	h := New(svc).Routes(auth.New(true, "", "", ""), "processor-token")
	req := httptest.NewRequest(http.MethodGet, "/v1/s/"+link.Token, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/share-links", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("create share without auth: %d", rec.Code)
	}
}
