package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultDirName is the default directory name for coffer data
	DefaultDirName = ".coffer"
	// DBFileName is the SQLite database filename
	DBFileName = "vault.db"
	// SessionFileName stores the current session info
	SessionFileName = "session"
	// ConfigFileName stores user preferences
	ConfigFileName = "config"
)

// Config holds the configuration for coffer
type Config struct {
	// DataDir is the directory where coffer stores its data
	DataDir string
	// DBPath is the full path to the SQLite database
	DBPath string
	// SessionPath is the full path to the session file
	SessionPath string
}

// DefaultDataDir returns the default data directory (~/.coffer)
func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, DefaultDirName), nil
}

// New creates a new Config with the default data directory
func New() (*Config, error) {
	dataDir, err := DefaultDataDir()
	if err != nil {
		return nil, err
	}
	return NewWithDataDir(dataDir), nil
}

// NewWithDataDir creates a new Config with a custom data directory
func NewWithDataDir(dataDir string) *Config {
	return &Config{
		DataDir:     dataDir,
		DBPath:      filepath.Join(dataDir, DBFileName),
		SessionPath: filepath.Join(dataDir, SessionFileName),
	}
}

// EnsureDataDir creates the data directory if it doesn't exist
// Sets permissions to 0700 (owner read/write/execute only)
func (c *Config) EnsureDataDir() error {
	if err := os.MkdirAll(c.DataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	return nil
}

// Exists checks if the vault database exists
func (c *Config) Exists() bool {
	_, err := os.Stat(c.DBPath)
	return err == nil
}

// Remove deletes the entire data directory (use with caution!)
func (c *Config) Remove() error {
	return os.RemoveAll(c.DataDir)
}

// WriteSession writes session data to the session file
// Sets permissions to 0600 (owner read/write only)
func (c *Config) WriteSession(data []byte) error {
	if err := c.EnsureDataDir(); err != nil {
		return err
	}
	return os.WriteFile(c.SessionPath, data, 0600)
}

// ReadSession reads session data from the session file
func (c *Config) ReadSession() ([]byte, error) {
	return os.ReadFile(c.SessionPath)
}

// DeleteSession removes the session file
func (c *Config) DeleteSession() error {
	err := os.Remove(c.SessionPath)
	if os.IsNotExist(err) {
		return nil // Already deleted, not an error
	}
	return err
}

// SessionExists checks if a session file exists
func (c *Config) SessionExists() bool {
	_, err := os.Stat(c.SessionPath)
	return err == nil
}
