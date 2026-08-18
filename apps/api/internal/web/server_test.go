package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/portfolio/pf-media/api/internal/domain"
)

func TestWriteErrTooLarge(t *testing.T) {
	rec := httptest.NewRecorder()
	writeErr(rec, domain.ErrTooLarge)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
}
