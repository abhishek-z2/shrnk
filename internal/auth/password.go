package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      uint32 = 64 * 1024 //64 of ram required per hash
	argonIterations  uint32 = 3         //the algo. passes 3 times over the memory
	argonParallelism uint8  = 2
	saltLength              = 16
	keyLength               = 32 //final size of the hash
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argonIterations,
		argonMemory,
		argonParallelism,
		keyLength,
	)

	encode := base64.RawStdEncoding.EncodeToString // references the function that converts [] byte to base64

	encoded := fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		encode(salt),
		encode(hash),
	)

	return encoded, nil
}

func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")

	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false, fmt.Errorf("invalid password hash format")
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8

	if _, err := fmt.Sscanf(
		parts[2],
		"m=%d,t=%d,p=%d",
		&memory,
		&iterations,
		&parallelism,
	); err != nil {
		return false, fmt.Errorf("parse password hash parameters: %w", err)
	}

	if memory < 8*uint32(parallelism) || memory > 256*1024 || // sanity check; prevents malicious input from triggering out-of-memory or cpu crashes
		iterations == 0 || iterations > 10 ||
		parallelism == 0 || parallelism > 16 {
		return false, fmt.Errorf("password hash parameters out of bounds")
	}

	decode := base64.RawStdEncoding.DecodeString // references the function that converts base64 to [] byte

	salt, err := decode(parts[3])

	if err != nil {
		return false, fmt.Errorf("decode password salt: %w", err)
	}

	expectedHash, err := decode(parts[4])

	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}

	if len(salt) < 8 || len(salt) > 64 || //sanity check
		len(expectedHash) < 16 || len(expectedHash) > 64 {
		return false, fmt.Errorf("invalid password hash lengths")
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil // compares both strings completely even if corresponding bytes fail to match; to avoid timing attacks
}
