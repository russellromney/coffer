package crypto

import (
	"golang.org/x/crypto/argon2"
)

// Argon2id parameters
// These are recommended parameters for interactive logins
// See: https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-argon2-13#section-4
const (
	// ArgonTime is the number of iterations
	ArgonTime = 3
	// ArgonMemory is the memory usage in KiB (64 MB)
	ArgonMemory = 64 * 1024
	// ArgonThreads is the number of parallel threads
	ArgonThreads = 4
)

// DeriveKey derives a 256-bit encryption key from a password using Argon2id
// Argon2id is memory-hard and resistant to GPU/ASIC attacks
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		ArgonTime,
		ArgonMemory,
		ArgonThreads,
		KeyLength,
	)
}

// DeriveKeyWithParams derives a key with custom Argon2id parameters
// This is useful for testing with faster parameters
func DeriveKeyWithParams(password string, salt []byte, time, memory uint32, threads uint8) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		time,
		memory,
		threads,
		KeyLength,
	)
}
