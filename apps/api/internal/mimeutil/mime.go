package mimeutil

import "strings"

var allowed = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/gif":  {},
}

func AllowedImage(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	_, ok := allowed[ct]
	return ok
}

func IsImage(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	return strings.HasPrefix(ct, "image/")
}
