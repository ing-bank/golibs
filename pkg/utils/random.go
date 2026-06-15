package utils

import (
	"crypto/rand"
	"fmt"
)

// RandStr generates a random string, should only be used for unit tests
func RandStr() string {
	b := make([]byte, 256)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)[2:256]
}
