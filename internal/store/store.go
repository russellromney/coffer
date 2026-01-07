package store

import (
	"github.com/russellromney/coffer/internal/models"
)

// Store defines the interface for persistent storage
type Store interface {
	// Close closes the database connection
	Close() error

	// Vault operations
	GetVaultMeta() (*models.VaultMeta, error)
	CreateVaultMeta(salt, keyCheck, keyCheckNonce []byte) error
	SetKeychainEnabled(enabled bool) error

	// Project operations
	CreateProject(name, description string) (*models.Project, error)
	GetProject(id string) (*models.Project, error)
	GetProjectByName(name string) (*models.Project, error)
	ListProjects() ([]models.Project, error)
	DeleteProject(id string) error

	// Environment operations
	CreateEnvironment(projectID, name string) (*models.Environment, error)
	CreateEnvironmentWithParent(projectID, name, parentID string) (*models.Environment, error)
	GetEnvironment(id string) (*models.Environment, error)
	GetEnvironmentByName(projectID, name string) (*models.Environment, error)
	ListEnvironments(projectID string) ([]models.Environment, error)
	DeleteEnvironment(id string) error
	GetEnvironmentAncestors(envID string) ([]models.Environment, error)
	GetEnvironmentChildren(envID string) ([]models.Environment, error)

	// Secret operations
	CreateSecret(envID, key string, encryptedValue, nonce []byte) (*models.Secret, error)
	UpdateSecret(envID, key string, encryptedValue, nonce []byte) (*models.Secret, error)
	GetSecret(envID, key string) (*models.Secret, error)
	ListSecrets(envID string) ([]models.Secret, error)
	DeleteSecret(envID, key string) error

	// Inheritance-aware secret operations
	GetSecretWithInheritance(envID, key string) (*models.MergedSecret, error)
	ListSecretsWithInheritance(envID string) ([]models.MergedSecret, error)

	// Secret history operations
	GetSecretHistory(envID, key string, limit int) ([]models.SecretHistory, error)
	GetSecretVersion(envID, key string, version int) (*models.SecretHistory, error)

	// Config operations
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error
	DeleteConfig(key string) error

	// Audit operations
	LogAudit(log *models.AuditLog) error
	GetAuditLogs(limit int) ([]models.AuditLog, error)
}
