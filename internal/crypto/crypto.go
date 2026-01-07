package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	// KeyLength is the length of the encryption key in bytes (256 bits)
	KeyLength = 32
	// NonceLength is the length of the GCM nonce in bytes (96 bits)
	NonceLength = 12
	// SaltLength is the length of the salt for key derivation in bytes
	SaltLength = 16
)

var (
	// ErrInvalidKeyLength is returned when the key is not the correct length
	ErrInvalidKeyLength = errors.New("invalid key length: must be 32 bytes")
	// ErrInvalidNonceLength is returned when the nonce is not the correct length
	ErrInvalidNonceLength = errors.New("invalid nonce length: must be 12 bytes")
	// ErrDecryptionFailed is returned when decryption fails (wrong key or corrupted data)
	ErrDecryptionFailed = errors.New("decryption failed: wrong key or corrupted data")
)

// GenerateKey generates a random 256-bit encryption key
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeyLength)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// GenerateSalt generates a random salt for key derivation
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateNonce generates a random nonce for GCM encryption
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, NonceLength)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return nonce, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
// The aad (additional authenticated data) is optional and can be used to bind
// the ciphertext to a specific context (e.g., the secret key name)
func Encrypt(key, plaintext, aad []byte) (ciphertext, nonce []byte, err error) {
	if len(key) != KeyLength {
		return nil, nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce, err = GenerateNonce()
	if err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, aad)
	return ciphertext, nonce, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
// The aad must match what was used during encryption
func Decrypt(key, ciphertext, nonce, aad []byte) ([]byte, error) {
	if len(key) != KeyLength {
		return nil, ErrInvalidKeyLength
	}
	if len(nonce) != NonceLength {
		return nil, ErrInvalidNonceLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptString is a convenience wrapper for Encrypt that works with strings
func EncryptString(key []byte, plaintext string, aad []byte) (ciphertext, nonce []byte, err error) {
	return Encrypt(key, []byte(plaintext), aad)
}

// DecryptString is a convenience wrapper for Decrypt that returns a string
func DecryptString(key, ciphertext, nonce, aad []byte) (string, error) {
	plaintext, err := Decrypt(key, ciphertext, nonce, aad)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
