package telemetry

import (
	"context"
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
