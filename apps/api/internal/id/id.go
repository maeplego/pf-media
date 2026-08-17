package id

import (
	"fmt"

	"github.com/oklog/ulid/v2"
)

func New() string {
	return ulid.Make().String()
}

func Parse(s string) error {
	if _, err := ulid.Parse(s); err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}
	return nil
}
