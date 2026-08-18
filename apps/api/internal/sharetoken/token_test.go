package sharetoken

import (
	"encoding/base64"
	"testing"
)

func TestNewUnpredictable(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 64; i++ {
		tok, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) < 43 {
			t.Fatalf("token too short: %q", tok)
		}
		if _, err := base64.RawURLEncoding.DecodeString(tok); err != nil {
			t.Fatalf("not url unpadded base64: %v", err)
		}
		if _, ok := seen[tok]; ok {
			t.Fatal("duplicate token")
		}
		seen[tok] = struct{}{}
	}
}
