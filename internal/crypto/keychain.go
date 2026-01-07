package crypto

import (
	"encoding/base64"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	// KeychainService is the service name used in the OS keychain
	KeychainService = "coffer"
	// KeychainAccount is the account name for the master key
	KeychainAccount = "master-key"
)

// KeychainAvailable checks if the OS keychain is available
func KeychainAvailable() bool {
	// Try to access keychain - if it fails, keychain is not available
	_, err := keyring.Get(KeychainService, "__test__")
	// ErrNotFound means keychain works but key doesn't exist (that's fine)
	// Any other error means keychain isn't available
	return err == keyring.ErrNotFound || err == nil
}

// StoreKeyInKeychain stores the derived encryption key in the OS keychain
func StoreKeyInKeychain(key []byte) error {
	if len(key) != KeyLength {
		return ErrInvalidKeyLength
	}

	// Encode key as base64 for storage
	encoded := base64.StdEncoding.EncodeToString(key)

	err := keyring.Set(KeychainService, KeychainAccount, encoded)
	if err != nil {
		return fmt.Errorf("failed to store key in keychain: %w", err)
	}

	return nil
}

// GetKeyFromKeychain retrieves the encryption key from the OS keychain
func GetKeyFromKeychain() ([]byte, error) {
	encoded, err := keyring.Get(KeychainService, KeychainAccount)
	if err == keyring.ErrNotFound {
		return nil, fmt.Errorf("no key found in keychain")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key from keychain: %w", err)
	}

	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key from keychain: %w", err)
	}

	if len(key) != KeyLength {
		return nil, fmt.Errorf("invalid key length in keychain")
	}

	return key, nil
}

// DeleteKeyFromKeychain removes the encryption key from the OS keychain
func DeleteKeyFromKeychain() error {
	err := keyring.Delete(KeychainService, KeychainAccount)
	if err == keyring.ErrNotFound {
		return nil // Already deleted, not an error
	}
	if err != nil {
		return fmt.Errorf("failed to delete key from keychain: %w", err)
	}
	return nil
}

// HasKeyInKeychain checks if a key exists in the keychain
func HasKeyInKeychain() bool {
	_, err := keyring.Get(KeychainService, KeychainAccount)
	return err == nil
}
