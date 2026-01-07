package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDataDir(t *testing.T) {
	dir, err := DefaultDataDir()
	if err != nil {
		t.Fatalf("DefaultDataDir() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, DefaultDirName)
	if dir != expected {
		t.Errorf("DefaultDataDir() = %v, want %v", dir, expected)
	}
}

func TestNew(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if cfg.DataDir == "" {
		t.Error("New() DataDir is empty")
	}
	if cfg.DBPath == "" {
		t.Error("New() DBPath is empty")
	}
	if cfg.SessionPath == "" {
		t.Error("New() SessionPath is empty")
	}
}

func TestNewWithDataDir(t *testing.T) {
	customDir := "/tmp/test-coffer"
	cfg := NewWithDataDir(customDir)

	if cfg.DataDir != customDir {
		t.Errorf("NewWithDataDir() DataDir = %v, want %v", cfg.DataDir, customDir)
	}
	if cfg.DBPath != filepath.Join(customDir, DBFileName) {
		t.Errorf("NewWithDataDir() DBPath = %v, want %v", cfg.DBPath, filepath.Join(customDir, DBFileName))
	}
	if cfg.SessionPath != filepath.Join(customDir, SessionFileName) {
		t.Errorf("NewWithDataDir() SessionPath = %v, want %v", cfg.SessionPath, filepath.Join(customDir, SessionFileName))
	}
}

func TestEnsureDataDir(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-coffer")
	cfg := NewWithDataDir(testDir)

	// Directory shouldn't exist yet
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Fatal("Test directory already exists")
	}

	// Create the directory
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() error = %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}

	// Verify permissions (0700)
	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("Directory permissions = %o, want 0700", perm)
	}

	// Calling again should not error
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() second call error = %v", err)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewWithDataDir(tmpDir)

	// Should not exist initially
	if cfg.Exists() {
		t.Error("Exists() = true, want false (DB doesn't exist)")
	}

	// Create the DB file
	if err := os.WriteFile(cfg.DBPath, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test DB file: %v", err)
	}

	// Should exist now
	if !cfg.Exists() {
		t.Error("Exists() = false, want true (DB exists)")
	}
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-coffer")
	cfg := NewWithDataDir(testDir)

	// Create directory and some files
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() error = %v", err)
	}
	if err := os.WriteFile(cfg.DBPath, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Remove everything
	if err := cfg.Remove(); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Error("Remove() did not delete directory")
	}
}

func TestSession(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := NewWithDataDir(tmpDir)

	// Session shouldn't exist initially
	if cfg.SessionExists() {
		t.Error("SessionExists() = true, want false")
	}

	// Write session
	sessionData := []byte("test-session-data")
	if err := cfg.WriteSession(sessionData); err != nil {
		t.Fatalf("WriteSession() error = %v", err)
	}

	// Session should exist now
	if !cfg.SessionExists() {
		t.Error("SessionExists() = false, want true")
	}

	// Read session
	readData, err := cfg.ReadSession()
	if err != nil {
		t.Fatalf("ReadSession() error = %v", err)
	}
	if string(readData) != string(sessionData) {
		t.Errorf("ReadSession() = %v, want %v", string(readData), string(sessionData))
	}

	// Verify session file permissions (0600)
	info, err := os.Stat(cfg.SessionPath)
	if err != nil {
		t.Fatalf("Failed to stat session file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Session file permissions = %o, want 0600", perm)
	}

	// Delete session
	if err := cfg.DeleteSession(); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	// Session should not exist anymore
	if cfg.SessionExists() {
		t.Error("SessionExists() = true after delete, want false")
	}

	// Deleting again should not error
	if err := cfg.DeleteSession(); err != nil {
		t.Fatalf("DeleteSession() second call error = %v", err)
	}
}
