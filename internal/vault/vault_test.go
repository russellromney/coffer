package vault

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/russellromney/coffer/internal/config"
	"github.com/russellromney/coffer/internal/store"
)

func setupTestVault(t *testing.T) (*Vault, *config.Config) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.NewWithDataDir(tmpDir)
	v := New(cfg)

	t.Cleanup(func() {
		v.Close()
	})

	return v, cfg
}

func TestIsInitialized(t *testing.T) {
	v, _ := setupTestVault(t)

	// Should not be initialized initially
	if v.IsInitialized() {
		t.Error("IsInitialized() = true, want false for new vault")
	}

	// Initialize
	err := v.Initialize("test-password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Should be initialized now
	if !v.IsInitialized() {
		t.Error("IsInitialized() = false, want true after initialization")
	}
}

func TestInitialize(t *testing.T) {
	v, cfg := setupTestVault(t)

	err := v.Initialize("test-password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Verify vault database was created
	if !cfg.Exists() {
		t.Error("Vault database was not created")
	}

	// Verify session was created
	if !cfg.SessionExists() {
		t.Error("Session was not created after initialization")
	}

	// Should be unlocked after initialization
	if !v.IsUnlocked() {
		t.Error("Vault should be unlocked after initialization")
	}
}

func TestInitializeAlreadyInitialized(t *testing.T) {
	v, _ := setupTestVault(t)

	// First initialization should succeed
	err := v.Initialize("test-password")
	if err != nil {
		t.Fatalf("First Initialize() error = %v", err)
	}

	// Second initialization should fail
	err = v.Initialize("another-password")
	if err != ErrAlreadyInitialized {
		t.Errorf("Second Initialize() error = %v, want ErrAlreadyInitialized", err)
	}
}

func TestUnlock(t *testing.T) {
	v, _ := setupTestVault(t)

	// Initialize with password
	err := v.Initialize("my-secret-password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Lock the vault
	err = v.Lock()
	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	// Should be locked
	if v.IsUnlocked() {
		t.Error("IsUnlocked() = true after Lock(), want false")
	}

	// Unlock with correct password
	err = v.Unlock("my-secret-password")
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	// Should be unlocked
	if !v.IsUnlocked() {
		t.Error("IsUnlocked() = false after Unlock(), want true")
	}
}

func TestUnlockWrongPassword(t *testing.T) {
	v, _ := setupTestVault(t)

	err := v.Initialize("correct-password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	v.Lock()

	// Try to unlock with wrong password
	err = v.Unlock("wrong-password")
	if err != ErrInvalidPassword {
		t.Errorf("Unlock() with wrong password error = %v, want ErrInvalidPassword", err)
	}

	// Should still be locked
	if v.IsUnlocked() {
		t.Error("IsUnlocked() = true after failed unlock, want false")
	}
}

func TestUnlockNotInitialized(t *testing.T) {
	v, _ := setupTestVault(t)

	err := v.Unlock("any-password")
	if err != ErrNotInitialized {
		t.Errorf("Unlock() on uninitialized vault error = %v, want ErrNotInitialized", err)
	}
}

func TestLock(t *testing.T) {
	v, cfg := setupTestVault(t)

	err := v.Initialize("password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Session should exist
	if !cfg.SessionExists() {
		t.Error("Session should exist after initialization")
	}

	// Lock the vault
	err = v.Lock()
	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	// Session should be deleted
	if cfg.SessionExists() {
		t.Error("Session should not exist after Lock()")
	}

	// IsUnlocked should return false
	if v.IsUnlocked() {
		t.Error("IsUnlocked() = true after Lock(), want false")
	}
}

func TestGetKey(t *testing.T) {
	v, _ := setupTestVault(t)

	err := v.Initialize("password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Get key while unlocked
	key, err := v.GetKey()
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	if len(key) != 32 {
		t.Errorf("GetKey() key length = %d, want 32", len(key))
	}

	// Lock and try to get key
	v.Lock()

	_, err = v.GetKey()
	if err != ErrLocked {
		t.Errorf("GetKey() while locked error = %v, want ErrLocked", err)
	}
}

func TestGetKeyConsistency(t *testing.T) {
	v, _ := setupTestVault(t)

	password := "test-password"
	err := v.Initialize(password)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Get key
	key1, err := v.GetKey()
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	// Lock and unlock
	v.Lock()
	v.Unlock(password)

	// Get key again
	key2, err := v.GetKey()
	if err != nil {
		t.Fatalf("GetKey() after re-unlock error = %v", err)
	}

	// Keys should be the same (derived from same password + salt)
	if string(key1) != string(key2) {
		t.Error("Keys should be consistent across unlock sessions")
	}
}

func TestVerifyPassword(t *testing.T) {
	v, _ := setupTestVault(t)

	password := "my-password"
	err := v.Initialize(password)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	v.Lock()

	// Verify correct password
	err = v.VerifyPassword(password)
	if err != nil {
		t.Errorf("VerifyPassword() with correct password error = %v", err)
	}

	// Verify wrong password
	err = v.VerifyPassword("wrong-password")
	if err != ErrInvalidPassword {
		t.Errorf("VerifyPassword() with wrong password error = %v, want ErrInvalidPassword", err)
	}

	// Vault should still be locked (VerifyPassword doesn't unlock)
	if v.IsUnlocked() {
		t.Error("VerifyPassword() should not unlock the vault")
	}
}

func TestVerifyPasswordNotInitialized(t *testing.T) {
	v, _ := setupTestVault(t)

	err := v.VerifyPassword("any")
	if err != ErrNotInitialized {
		t.Errorf("VerifyPassword() on uninitialized vault error = %v, want ErrNotInitialized", err)
	}
}

func TestGetStore(t *testing.T) {
	v, _ := setupTestVault(t)

	err := v.Initialize("password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	store, err := v.GetStore()
	if err != nil {
		t.Fatalf("GetStore() error = %v", err)
	}

	if store == nil {
		t.Error("GetStore() returned nil store")
	}

	// Should be able to use the store
	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("store.ListProjects() error = %v", err)
	}
	if projects == nil {
		t.Error("store.ListProjects() returned nil")
	}
}

func TestSessionExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.NewWithDataDir(tmpDir)
	v := New(cfg)
	defer v.Close()

	err := v.Initialize("password")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Manually create an expired session
	expiredSession := Session{
		Key:       make([]byte, 32),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	data, _ := json.Marshal(expiredSession)
	cfg.WriteSession(data)

	// Should not be unlocked
	if v.IsUnlocked() {
		t.Error("IsUnlocked() = true with expired session, want false")
	}

	// GetKey should fail
	_, err = v.GetKey()
	if err != ErrSessionExpired {
		t.Errorf("GetKey() with expired session error = %v, want ErrSessionExpired", err)
	}
}

func TestMultipleVaults(t *testing.T) {
	// Test that two vaults with different data dirs are independent
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	cfg1 := config.NewWithDataDir(dir1)
	cfg2 := config.NewWithDataDir(dir2)

	v1 := New(cfg1)
	v2 := New(cfg2)
	defer v1.Close()
	defer v2.Close()

	// Initialize with different passwords
	v1.Initialize("password1")
	v2.Initialize("password2")

	// Each should be independently unlocked
	if !v1.IsUnlocked() {
		t.Error("v1 should be unlocked")
	}
	if !v2.IsUnlocked() {
		t.Error("v2 should be unlocked")
	}

	// Lock v1, v2 should still be unlocked
	v1.Lock()
	if v1.IsUnlocked() {
		t.Error("v1 should be locked")
	}
	if !v2.IsUnlocked() {
		t.Error("v2 should still be unlocked")
	}

	// Unlock v1 with its password
	err := v1.Unlock("password1")
	if err != nil {
		t.Errorf("v1.Unlock() error = %v", err)
	}

	// v1's password shouldn't work for v2
	v2.Lock()
	err = v2.Unlock("password1")
	if err != ErrInvalidPassword {
		t.Errorf("v2.Unlock() with v1's password error = %v, want ErrInvalidPassword", err)
	}
}

func TestVaultPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	password := "persistent-password"

	// Create and initialize vault
	cfg1 := config.NewWithDataDir(tmpDir)
	v1 := New(cfg1)

	err := v1.Initialize(password)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Get the key
	key1, _ := v1.GetKey()

	// Close the vault
	v1.Close()

	// Create a new vault instance pointing to the same directory
	cfg2 := config.NewWithDataDir(tmpDir)
	v2 := New(cfg2)
	defer v2.Close()

	// Should be initialized
	if !v2.IsInitialized() {
		t.Error("New vault instance should see existing initialization")
	}

	// Should be unlocked (session file persists)
	if !v2.IsUnlocked() {
		t.Error("New vault instance should see existing session")
	}

	// Key should be the same
	key2, _ := v2.GetKey()
	if string(key1) != string(key2) {
		t.Error("Key should be persistent across vault instances")
	}
}

func TestNewWithStore(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.NewWithDataDir(tmpDir)
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store manually
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer s.Close()

	// Create vault with existing store
	v := NewWithStore(cfg, s)

	// Store should be used
	gotStore, err := v.GetStore()
	if err != nil {
		t.Fatalf("GetStore() error = %v", err)
	}
	if gotStore != s {
		t.Error("GetStore() should return the injected store")
	}
}
