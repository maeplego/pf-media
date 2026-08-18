package password

import "testing"

func TestHashVerify(t *testing.T) {
	h, err := Hash("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if h == "correct-horse" || h == "" {
		t.Fatal("must not store plaintext")
	}
	ok, err := Verify("correct-horse", h)
	if err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	ok, err = Verify("wrong", h)
	if err != nil || ok {
		t.Fatalf("expected mismatch, ok=%v err=%v", ok, err)
	}
}

func TestHashRejectsEmpty(t *testing.T) {
	if _, err := Hash(""); err == nil {
		t.Fatal("empty password must not hash")
	}
}
