package telemetry

import (
	"context"
	"net/http"
	"testing"
)

func TestInitNoopWithoutEndpoint(t *testing.T) {
	shutdown, err := Init(context.Background(), "media-api", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSkipProbe(t *testing.T) {
	health, _ := http.NewRequest(http.MethodGet, "http://api/health", nil)
	if SkipProbe(health) {
		t.Fatal("health should be skipped")
	}
	presign, _ := http.NewRequest(http.MethodPost, "http://api/v1/uploads/presign", nil)
	if !SkipProbe(presign) {
		t.Fatal("presign should be traced")
	}
}
