package mimeutil

import "testing"

func TestAllowedImage(t *testing.T) {
	if !AllowedImage("image/jpeg") || AllowedImage("application/pdf") {
		t.Fatal()
	}
}
