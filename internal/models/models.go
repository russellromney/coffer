package models

import "time"

// Project represents a group of environments and secrets
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Environment represents a deployment environment within a project (dev, staging, prod)
type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	ParentID  *string   `json:"parent_id,omitempty"` // nil for root environments
	CreatedAt time.Time `json:"created_at"`
}

// Secret represents an encrypted secret value
type Secret struct {
	ID             string    `json:"id"`
	EnvironmentID  string    `json:"environment_id"`
	Key            string    `json:"key"`
	EncryptedValue []byte    `json:"-"` // Never serialize
	Nonce          []byte    `json:"-"` // Never serialize
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// MergedSecret represents a secret with its source environment info (for inheritance)
type MergedSecret struct {
	Secret
	SourceEnvID   string `json:"source_env_id"`   // Environment where secret is defined
	SourceEnvName string `json:"source_env_name"` // Environment name for display
	IsInherited   bool   `json:"is_inherited"`    // true if from parent, false if local
}

// SecretHistory records changes to secrets for versioning
type SecretHistory struct {
	ID             string    `json:"id"`
	EnvironmentID  string    `json:"environment_id"`
	Key            string    `json:"key"`
	EncryptedValue []byte    `json:"-"`
	Nonce          []byte    `json:"-"`
	Version        int       `json:"version"`
	ChangeType     string    `json:"change_type"` // "create", "update", "delete"
	CreatedAt      time.Time `json:"created_at"`
}

// VaultMeta stores vault-level metadata for password verification
type VaultMeta struct {
	ID              int       `json:"id"`
	Salt            []byte    `json:"-"`
	KeyCheck        []byte    `json:"-"` // Encrypted known value for verification
	KeyCheckNonce   []byte    `json:"-"`
	KeychainEnabled bool      `json:"keychain_enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

// AuditLog records actions taken on secrets
type AuditLog struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Action        string    `json:"action"` // "read", "create", "update", "delete", "export", "run"
	ProjectID     string    `json:"project_id,omitempty"`
	EnvironmentID string    `json:"environment_id,omitempty"`
	SecretKey     string    `json:"secret_key,omitempty"` // Key name only, never the value
	Success       bool      `json:"success"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// Config represents a key-value configuration setting
type Config struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Known configuration keys
const (
	ConfigActiveProject = "active_project"
)

// ChangeType constants
const (
	ChangeTypeCreate = "create"
	ChangeTypeUpdate = "update"
	ChangeTypeDelete = "delete"
)

// Action constants for audit log
const (
	ActionRead   = "read"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionExport = "export"
	ActionRun    = "run"
	ActionImport = "import"
)
