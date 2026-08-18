package sharetoken

import (
	"crypto/rand"
	"encoding/base64"
)

// rawBytes is 256 bits so a leaked prefix still is not enumerable.
const rawBytes = 32

func New() (string, error) {
	b := make([]byte, rawBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
