package mimeutil

import (
	"bytes"
	"strings"
)

var allowedImages = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/gif":  {},
}

var allowedDocuments = map[string]struct{}{
	"application/pdf": {},
	"application/zip": {},
	"text/plain":      {},
}

func normalize(ct string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
}

func AllowedImage(ct string) bool {
	_, ok := allowedImages[normalize(ct)]
	return ok
}

func AllowedDocument(ct string) bool {
	_, ok := allowedDocuments[normalize(ct)]
	return ok
}

// AllowedUpload is the presign whitelist (images + storage-only documents).
func AllowedUpload(ct string) bool {
	ct = normalize(ct)
	if _, ok := allowedImages[ct]; ok {
		return true
	}
	_, ok := allowedDocuments[ct]
	return ok
}

func IsImage(ct string) bool {
	return strings.HasPrefix(normalize(ct), "image/")
}

// SniffUpload detects MIME from the first bytes of an uploaded object.
func SniffUpload(head []byte) (string, bool) {
	if len(head) < 4 {
		return "", false
	}
	if len(head) >= 5 && bytes.HasPrefix(head, []byte("%PDF-")) {
		return "application/pdf", true
	}
	if head[0] == 'P' && head[1] == 'K' && (head[2] == 3 || head[2] == 5 || head[2] == 7) {
		return "application/zip", true
	}
	if len(head) >= 3 && head[0] == 0xff && head[1] == 0xd8 && head[2] == 0xff {
		return "image/jpeg", true
	}
	if len(head) >= 8 && bytes.Equal(head[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png", true
	}
	if len(head) >= 6 {
		gif := string(head[:6])
		if gif == "GIF87a" || gif == "GIF89a" {
			return "image/gif", true
		}
	}
	if len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP" {
		return "image/webp", true
	}
	if AllowedDocument("text/plain") && looksLikeText(head) {
		return "text/plain", true
	}
	return "", false
}

func looksLikeText(head []byte) bool {
	if len(head) == 0 {
		return false
	}
	for _, b := range head {
		if b == 0 {
			return false
		}
		if b < 0x09 || (b > 0x0d && b < 0x20 && b != 0x1b) {
			return false
		}
	}
	return true
}

// MatchesUpload returns whether object bytes match the declared Content-Type.
func MatchesUpload(declared string, head []byte) bool {
	declared = normalize(declared)
	if !AllowedUpload(declared) {
		return false
	}
	sniffed, ok := SniffUpload(head)
	if !ok {
		return false
	}
	return sniffed == declared
}
