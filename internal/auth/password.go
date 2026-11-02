// Package auth holds the password, rate-limiting and account-recovery logic.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters, following the OWASP recommendation for argon2id
// (19 MiB, 2 iterations, 1 lane).
const (
	argonMemoryKiB = 19 * 1024
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16
)

// hashSlots caps how many password hashes run at once. Each one briefly
// needs about 19 MiB, so this keeps peak memory small however many people
// press "Sign in" together.
var hashSlots = make(chan struct{}, 2)

var b64 = base64.RawStdEncoding

// HashPassword returns the argon2id hash of password in the standard
// "$argon2id$v=19$m=...,t=...,p=...$salt$hash" format.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hashSlots <- struct{}{}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	<-hashSlots

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// ErrInvalidHash means a stored hash could not be parsed.
var ErrInvalidHash = errors.New("invalid password hash")

// VerifyPassword reports whether password matches the encoded hash. The
// parameters are read from the hash, so old hashes keep working if the
// defaults above change.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrInvalidHash
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, ErrInvalidHash
	}
	if memory == 0 || time == 0 || threads == 0 || memory > 1<<20 {
		return false, ErrInvalidHash
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, ErrInvalidHash
	}

	hashSlots <- struct{}{}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	<-hashSlots

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
