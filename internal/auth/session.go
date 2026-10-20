package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

const sessionTokenLength = 32

func GenerateSessionToken() (string, error) {
	b := make([]byte, sessionTokenLength)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func HashSessionToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
