// Package password hashes share-link secrets with Argon2id in PHC format.
// Parameters live in the encoded string so cost can be raised without rewriting every row.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	algo      = "argon2id"
	version   = argon2.Version
	memoryKiB = 64 * 1024
	timeCost  = 1
	threads   = 4
	keyLen    = 32
	saltLen   = 16
)

func Hash(plain string) (string, error) {
	if plain == "" {
		return "", fmt.Errorf("password must not be empty")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}
	sum := argon2.IDKey([]byte(plain), salt, timeCost, memoryKiB, threads, keyLen)
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		algo, version, memoryKiB, timeCost, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

func Verify(plain, encoded string) (bool, error) {
	if plain == "" || encoded == "" {
		return false, nil
	}
	salt, want, mem, tcost, pars, err := parsePHC(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(plain), salt, tcost, mem, pars, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func parsePHC(encoded string) (salt, hash []byte, memory uint32, timeCost uint32, threads uint8, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != algo {
		return nil, nil, 0, 0, 0, fmt.Errorf("unsupported password hash format")
	}
	if parts[2] != fmt.Sprintf("v=%d", version) {
		return nil, nil, 0, 0, 0, fmt.Errorf("unsupported argon2 version")
	}
	var p uint64
	for _, kv := range strings.Split(parts[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, nil, 0, 0, 0, fmt.Errorf("invalid phc param %q", kv)
		}
		n, convErr := strconv.ParseUint(v, 10, 32)
		if convErr != nil {
			return nil, nil, 0, 0, 0, fmt.Errorf("invalid phc value: %w", convErr)
		}
		switch k {
		case "m":
			memory = uint32(n)
		case "t":
			timeCost = uint32(n)
		case "p":
			p = n
		}
	}
	if memory == 0 || timeCost == 0 || p == 0 || p > 255 {
		return nil, nil, 0, 0, 0, fmt.Errorf("incomplete phc parameters")
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, 0, 0, 0, fmt.Errorf("salt: %w", err)
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, 0, 0, 0, fmt.Errorf("hash: %w", err)
	}
	return salt, hash, memory, timeCost, uint8(p), nil
}
