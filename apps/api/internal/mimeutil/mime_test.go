package mimeutil

import "testing"

func TestAllowedUpload(t *testing.T) {
	if !AllowedUpload("image/jpeg") || !AllowedUpload("application/pdf") {
		t.Fatal("expected allowed")
	}
	if AllowedUpload("video/mp4") || AllowedUpload("application/octet-stream") {
		t.Fatal("expected rejected")
	}
}

func TestSniffPDF(t *testing.T) {
	ct, ok := SniffUpload([]byte("%PDF-1.4\n"))
	if !ok || ct != "application/pdf" {
		t.Fatalf("got %q %v", ct, ok)
	}
	if !MatchesUpload("application/pdf", []byte("%PDF-1.4\n")) {
		t.Fatal("pdf should match")
	}
	if MatchesUpload("image/png", []byte("%PDF-1.4\n")) {
		t.Fatal("png should not match pdf bytes")
	}
}

func TestSniffPNG(t *testing.T) {
	head := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	ct, ok := SniffUpload(head)
	if !ok || ct != "image/png" {
		t.Fatalf("got %q %v", ct, ok)
	}
}

func TestSniffPlainText(t *testing.T) {
	if !MatchesUpload("text/plain", []byte("hello drive\n")) {
		t.Fatal("plain text should match")
	}
	if MatchesUpload("text/plain", []byte{0x00, 0x01}) {
		t.Fatal("binary should not match text/plain")
	}
}
