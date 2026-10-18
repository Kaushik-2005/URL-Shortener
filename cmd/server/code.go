package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func generateCode() (string, error) {
	randomBytes := make([]byte, 6)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate short code: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}
