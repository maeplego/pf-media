package objectstore

import "testing"

func TestObjectKeyLayout(t *testing.T) {
	if got := ObjectKey("alice", "01FILE"); got != "user/alice/01FILE/orig" {
		t.Fatalf("ObjectKey %s", got)
	}
	if got := ObjectPrefix("alice", "01FILE"); got != "user/alice/01FILE/" {
		t.Fatalf("ObjectPrefix %s", got)
	}
	if got := VariantKey("alice", "01FILE", "thumb"); got != "user/alice/01FILE/thumb" {
		t.Fatalf("VariantKey %s", got)
	}
}
