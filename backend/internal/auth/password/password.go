// Package password implements the Argon2id password-hashing policy.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/jorgepaez/identity-hub/internal/config"
)

const (
	saltLength       = 16
	keyLength        = 32
	maxPasswordRunes = 128
)

var ErrMalformedHash = errors.New("malformed Argon2id hash")

// InvalidPasswordError reports a password that cannot be hashed or verified.
type InvalidPasswordError struct {
	Reason string
}

func (e *InvalidPasswordError) Error() string {
	return "invalid password: " + e.Reason
}

type settings struct {
	params config.PasswordConfig
	sem    chan struct{}
}

var current = settingsFor(config.PasswordConfig{
	MemoryKiB: 65536, Iterations: 3, Parallelism: 2, Concurrency: 4,
})
var settingsMu sync.RWMutex
var decoyOnce sync.Once
var decoyHash string
var decoyErr error

func settingsFor(params config.PasswordConfig) settings {
	return settings{params: params, sem: make(chan struct{}, params.Concurrency)}
}

// Configure applies password settings loaded from configuration. The returned
// function restores the previous settings and is intended for process setup tests.
func Configure(params config.PasswordConfig) func() {
	if params.MemoryKiB == 0 || params.Iterations == 0 || params.Parallelism == 0 || params.Concurrency <= 0 {
		panic("password configuration must contain positive values")
	}
	settingsMu.Lock()
	previous := current
	current = settingsFor(params)
	settingsMu.Unlock()
	return func() {
		settingsMu.Lock()
		current = previous
		settingsMu.Unlock()
	}
}

// Hash creates a PHC-formatted Argon2id password hash.
func Hash(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", fmt.Errorf("validate password: %w", err)
	}
	return hash(password, snapshot())
}

// Verify compares password to an encoded Argon2id PHC hash in constant time.
func Verify(password, encodedHash string) (bool, error) {
	if err := validatePassword(password); err != nil {
		return false, fmt.Errorf("validate password: %w", err)
	}
	parsed, err := parse(encodedHash)
	if err != nil {
		return false, fmt.Errorf("parse password hash: %w", err)
	}
	state := snapshot()
	state.sem <- struct{}{}
	defer func() { <-state.sem }()
	actual := argon2.IDKey([]byte(password), parsed.salt, parsed.memory, parsed.iterations, parsed.parallelism, keyLength)
	return subtle.ConstantTimeCompare(actual, parsed.hash) == 1, nil
}

// VerifyDecoy performs a verification against a one-time generated decoy hash.
// Call it when a user lookup did not find a matching account.
func VerifyDecoy(password string) error {
	decoyOnce.Do(func() {
		decoyHash, decoyErr = hash("identity-hub-decoy-password", snapshot())
	})
	if decoyErr != nil {
		return fmt.Errorf("generate decoy password hash: %w", decoyErr)
	}
	_, err := Verify(password, decoyHash)
	if err != nil {
		return fmt.Errorf("verify decoy password hash: %w", err)
	}
	return nil
}

// NeedsRehash reports whether a valid hash uses non-current Argon2id settings.
func NeedsRehash(encodedHash string) bool {
	parsed, err := parse(encodedHash)
	if err != nil {
		return true
	}
	params := snapshot().params
	return parsed.memory != params.MemoryKiB || parsed.iterations != params.Iterations || parsed.parallelism != params.Parallelism || len(parsed.salt) != saltLength || len(parsed.hash) != keyLength
}

func hash(password string, state settings) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read password salt: %w", err)
	}
	state.sem <- struct{}{}
	defer func() { <-state.sem }()
	key := argon2.IDKey([]byte(password), salt, state.params.MemoryKiB, state.params.Iterations, state.params.Parallelism, keyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", state.params.MemoryKiB, state.params.Iterations, state.params.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func snapshot() settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return current
}

type parsedHash struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	hash        []byte
}

func parse(encoded string) (parsedHash, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return parsedHash{}, ErrMalformedHash
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return parsedHash{}, ErrMalformedHash
	}
	memory, err := parseUint(params[0], "m", 32)
	if err != nil {
		return parsedHash{}, err
	}
	iterations, err := parseUint(params[1], "t", 32)
	if err != nil {
		return parsedHash{}, err
	}
	parallelism, err := parseUint(params[2], "p", 8)
	if err != nil || memory == 0 || iterations == 0 || parallelism == 0 {
		return parsedHash{}, ErrMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return parsedHash{}, ErrMalformedHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) != keyLength {
		return parsedHash{}, ErrMalformedHash
	}
	return parsedHash{memory: uint32(memory), iterations: uint32(iterations), parallelism: uint8(parallelism), salt: salt, hash: hash}, nil
}

func parseUint(part, name string, bitSize int) (uint64, error) {
	prefix := name + "="
	if !strings.HasPrefix(part, prefix) {
		return 0, ErrMalformedHash
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(part, prefix), 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrMalformedHash, err)
	}
	return value, nil
}

func validatePassword(password string) error {
	if password == "" {
		return &InvalidPasswordError{Reason: "empty"}
	}
	if utf8.RuneCountInString(password) > maxPasswordRunes {
		return &InvalidPasswordError{Reason: "longer than 128 characters"}
	}
	return nil
}
