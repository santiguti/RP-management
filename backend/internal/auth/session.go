package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func NewSessionToken() (string, error) {
	return newRandomHexToken()
}

func HashSessionToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func NewCSRFToken() (string, error) {
	return newRandomHexToken()
}

func newRandomHexToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
