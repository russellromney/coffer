package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/crypto"
	"github.com/russellromney/coffer/internal/store"
)

const (
	// KeyCheckValue is the known plaintext used to verify the master password
	KeyCheckValue = "coffer-key-check-v1"
	// SessionDuration is how long a session remains valid
	SessionDuration = 8 * time.Hour
)

var (
	// ErrNotInitialized is returned when the vault has not been initialized
	ErrNotInitialized = errors.New("vault not initialized: run 'coffer init' first")
	// ErrAlreadyInitialized is returned when trying to init an existing vault
	ErrAlreadyInitialized = errors.New("vault already initialized")
	// ErrLocked is returned when the vault is locked
	ErrLocked = errors.New("vault is locked: run 'coffer unlock' first")
	// ErrInvalidPassword is returned when the password is incorrect
	ErrInvalidPassword = errors.New("invalid password")
	// ErrSessionExpired is returned when the session has expired
	ErrSessionExpired = errors.New("session expired: run 'coffer unlock' again")
	// ErrKeychainNotAvailable is returned when keychain is not available
	ErrKeychainNotAvailable = errors.New("keychain not available on this system")
	// ErrKeychainNotEnabled is returned when keychain is not enabled
	ErrKeychainNotEnabled = errors.New("keychain not enabled: use 'coffer keychain enable' first")
)

// Session represents an unlocked vault session
type Session struct {
	Key       []byte    `json:"key"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Vault manages the vault state and provides access to the encryption key
type Vault struct {
	cfg   *config.Config
	store store.Store
}

// New creates a new Vault instance
func New(cfg *config.Config) *Vault {
	return &Vault{cfg: cfg}
}

// NewWithStore creates a Vault with an existing store (useful for testing)
func NewWithStore(cfg *config.Config, s store.Store) *Vault {
	return &Vault{cfg: cfg, store: s}
}

// openStore opens or returns the existing store
func (v *Vault) openStore() (store.Store, error) {
	if v.store != nil {
		return v.store, nil
	}
	s, err := store.NewSQLiteStore(v.cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}
	v.store = s
	return s, nil
}

// Close closes the store if open
func (v *Vault) Close() error {
	if v.store != nil {
		err := v.store.Close()
		v.store = nil
		return err
	}
	return nil
}

// IsInitialized checks if the vault has been initialized
func (v *Vault) IsInitialized() bool {
	return v.cfg.Exists()
}

// Initialize creates a new vault with the given master password
func (v *Vault) Initialize(password string) error {
	if v.IsInitialized() {
		return ErrAlreadyInitialized
	}

	// Ensure data directory exists
	if err := v.cfg.EnsureDataDir(); err != nil {
		return err
	}

	// Generate salt for key derivation
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key from password
	key := crypto.DeriveKey(password, salt)

	// Encrypt the key check value to verify password later
	keyCheck, keyCheckNonce, err := crypto.Encrypt(key, []byte(KeyCheckValue), nil)
	if err != nil {
		return fmt.Errorf("failed to encrypt key check: %w", err)
	}

	// Open store and create vault metadata
	s, err := v.openStore()
	if err != nil {
		return err
	}

	if err := s.CreateVaultMeta(salt, keyCheck, keyCheckNonce); err != nil {
		return fmt.Errorf("failed to create vault metadata: %w", err)
	}

	// Create initial session
	if err := v.createSession(key); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// Unlock unlocks the vault with the given password
func (v *Vault) Unlock(password string) error {
	if !v.IsInitialized() {
		return ErrNotInitialized
	}

	s, err := v.openStore()
	if err != nil {
		return err
	}

	// Get vault metadata
	meta, err := s.GetVaultMeta()
	if err != nil {
		return fmt.Errorf("failed to get vault metadata: %w", err)
	}

	// Derive key from password
	key := crypto.DeriveKey(password, meta.Salt)

	// Verify key by decrypting the key check value
	plaintext, err := crypto.Decrypt(key, meta.KeyCheck, meta.KeyCheckNonce, nil)
	if err != nil {
		return ErrInvalidPassword
	}
	if string(plaintext) != KeyCheckValue {
		return ErrInvalidPassword
	}

	// Create session
	if err := v.createSession(key); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// Lock locks the vault by destroying the session
func (v *Vault) Lock() error {
	return v.cfg.DeleteSession()
}

// IsUnlocked checks if the vault is currently unlocked with a valid session
func (v *Vault) IsUnlocked() bool {
	session, err := v.loadSession()
	if err != nil {
		return false
	}
	return time.Now().Before(session.ExpiresAt)
}

// GetKey returns the encryption key if the vault is unlocked
func (v *Vault) GetKey() ([]byte, error) {
	session, err := v.loadSession()
	if err != nil {
		if errors.Is(err, ErrLocked) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		v.Lock() // Clean up expired session
		return nil, ErrSessionExpired
	}

	return session.Key, nil
}

// GetStore returns the store, opening it if necessary
func (v *Vault) GetStore() (store.Store, error) {
	return v.openStore()
}

// createSession creates a new session with the given key
func (v *Vault) createSession(key []byte) error {
	session := Session{
		Key:       key,
		ExpiresAt: time.Now().Add(SessionDuration),
	}

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Note: In production, you'd want to encrypt this session file
	// with a random session key or use OS keychain. For simplicity,
	// we store it with restrictive file permissions (0600).
	return v.cfg.WriteSession(data)
}

// loadSession loads the current session
func (v *Vault) loadSession() (*Session, error) {
	if !v.cfg.SessionExists() {
		return nil, ErrLocked
	}

	data, err := v.cfg.ReadSession()
	if err != nil {
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// VerifyPassword checks if the given password is correct without creating a session
func (v *Vault) VerifyPassword(password string) error {
	if !v.IsInitialized() {
		return ErrNotInitialized
	}

	s, err := v.openStore()
	if err != nil {
		return err
	}

	meta, err := s.GetVaultMeta()
	if err != nil {
		return fmt.Errorf("failed to get vault metadata: %w", err)
	}

	key := crypto.DeriveKey(password, meta.Salt)

	plaintext, err := crypto.Decrypt(key, meta.KeyCheck, meta.KeyCheckNonce, nil)
	if err != nil {
		return ErrInvalidPassword
	}
	if string(plaintext) != KeyCheckValue {
		return ErrInvalidPassword
	}

	return nil
}

// EnableKeychain stores the derived key in the OS keychain for passwordless unlock
func (v *Vault) EnableKeychain(password string) error {
	if !v.IsInitialized() {
		return ErrNotInitialized
	}

	if !crypto.KeychainAvailable() {
		return ErrKeychainNotAvailable
	}

	// Verify password first
	if err := v.VerifyPassword(password); err != nil {
		return err
	}

	s, err := v.openStore()
	if err != nil {
		return err
	}

	meta, err := s.GetVaultMeta()
	if err != nil {
		return fmt.Errorf("failed to get vault metadata: %w", err)
	}

	// Derive the key
	key := crypto.DeriveKey(password, meta.Salt)

	// Store in keychain
	if err := crypto.StoreKeyInKeychain(key); err != nil {
		return fmt.Errorf("failed to store key in keychain: %w", err)
	}

	// Update vault metadata
	if err := s.SetKeychainEnabled(true); err != nil {
		// Try to clean up keychain on failure
		crypto.DeleteKeyFromKeychain()
		return fmt.Errorf("failed to enable keychain: %w", err)
	}

	return nil
}

// DisableKeychain removes the key from the OS keychain
func (v *Vault) DisableKeychain() error {
	if !v.IsInitialized() {
		return ErrNotInitialized
	}

	s, err := v.openStore()
	if err != nil {
		return err
	}

	// Delete from keychain
	if err := crypto.DeleteKeyFromKeychain(); err != nil {
		return fmt.Errorf("failed to delete key from keychain: %w", err)
	}

	// Update vault metadata
	if err := s.SetKeychainEnabled(false); err != nil {
		return fmt.Errorf("failed to disable keychain: %w", err)
	}

	return nil
}

// UnlockWithKeychain unlocks the vault using the OS keychain
func (v *Vault) UnlockWithKeychain() error {
	if !v.IsInitialized() {
		return ErrNotInitialized
	}

	s, err := v.openStore()
	if err != nil {
		return err
	}

	// Check if keychain is enabled
	meta, err := s.GetVaultMeta()
	if err != nil {
		return fmt.Errorf("failed to get vault metadata: %w", err)
	}

	if !meta.KeychainEnabled {
		return ErrKeychainNotEnabled
	}

	// Get key from keychain
	key, err := crypto.GetKeyFromKeychain()
	if err != nil {
		return fmt.Errorf("failed to get key from keychain: %w", err)
	}

	// Verify key is correct
	plaintext, err := crypto.Decrypt(key, meta.KeyCheck, meta.KeyCheckNonce, nil)
	if err != nil {
		return fmt.Errorf("keychain key is invalid: %w", err)
	}
	if string(plaintext) != KeyCheckValue {
		return fmt.Errorf("keychain key is invalid")
	}

	// Create session
	if err := v.createSession(key); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// IsKeychainEnabled checks if keychain is enabled for this vault
func (v *Vault) IsKeychainEnabled() (bool, error) {
	if !v.IsInitialized() {
		return false, ErrNotInitialized
	}

	s, err := v.openStore()
	if err != nil {
		return false, err
	}

	meta, err := s.GetVaultMeta()
	if err != nil {
		return false, fmt.Errorf("failed to get vault metadata: %w", err)
	}

	return meta.KeychainEnabled, nil
}

// IsKeychainAvailable checks if OS keychain is available
func (v *Vault) IsKeychainAvailable() bool {
	return crypto.KeychainAvailable()
}
