package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/russellromney/coffer/internal/models"
	_ "modernc.org/sqlite"
)

var (
	// ErrNotFound is returned when a requested item doesn't exist
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists is returned when trying to create a duplicate
	ErrAlreadyExists = errors.New("already exists")
)

// SQLiteStore implements Store using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set busy timeout
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// migrate creates the database schema
func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS vault_meta (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		salt BLOB NOT NULL,
		key_check BLOB NOT NULL,
		key_check_nonce BLOB NOT NULL,
		keychain_enabled BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS environments (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		parent_id TEXT REFERENCES environments(id) ON DELETE RESTRICT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, name)
	);
	CREATE INDEX IF NOT EXISTS idx_environments_project ON environments(project_id);
	CREATE INDEX IF NOT EXISTS idx_environments_parent ON environments(parent_id);

	CREATE TABLE IF NOT EXISTS secrets (
		id TEXT PRIMARY KEY,
		environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
		key TEXT NOT NULL,
		encrypted_value BLOB NOT NULL,
		nonce BLOB NOT NULL,
		version INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(environment_id, key)
	);
	CREATE INDEX IF NOT EXISTS idx_secrets_env ON secrets(environment_id);
	CREATE INDEX IF NOT EXISTS idx_secrets_env_key ON secrets(environment_id, key);

	CREATE TABLE IF NOT EXISTS secret_history (
		id TEXT PRIMARY KEY,
		environment_id TEXT NOT NULL,
		key TEXT NOT NULL,
		encrypted_value BLOB NOT NULL,
		nonce BLOB NOT NULL,
		version INTEGER NOT NULL,
		change_type TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_secret_history_env_key ON secret_history(environment_id, key);

	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id TEXT PRIMARY KEY,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		action TEXT NOT NULL,
		project_id TEXT,
		environment_id TEXT,
		secret_key TEXT,
		success BOOLEAN DEFAULT 1,
		error_message TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Migration: Add parent_id column to existing environments table if it doesn't exist
	// This handles upgrades from older versions
	_, err = s.db.Exec(`
		ALTER TABLE environments ADD COLUMN parent_id TEXT REFERENCES environments(id) ON DELETE RESTRICT
	`)
	// Ignore error - column may already exist
	_ = err

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Vault operations

func (s *SQLiteStore) GetVaultMeta() (*models.VaultMeta, error) {
	var meta models.VaultMeta
	err := s.db.QueryRow(`
		SELECT id, salt, key_check, key_check_nonce, keychain_enabled, created_at
		FROM vault_meta WHERE id = 1
	`).Scan(&meta.ID, &meta.Salt, &meta.KeyCheck, &meta.KeyCheckNonce, &meta.KeychainEnabled, &meta.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get vault meta: %w", err)
	}
	return &meta, nil
}

func (s *SQLiteStore) CreateVaultMeta(salt, keyCheck, keyCheckNonce []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO vault_meta (id, salt, key_check, key_check_nonce)
		VALUES (1, ?, ?, ?)
	`, salt, keyCheck, keyCheckNonce)
	if err != nil {
		return fmt.Errorf("failed to create vault meta: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SetKeychainEnabled(enabled bool) error {
	_, err := s.db.Exec(`UPDATE vault_meta SET keychain_enabled = ? WHERE id = 1`, enabled)
	if err != nil {
		return fmt.Errorf("failed to set keychain enabled: %w", err)
	}
	return nil
}

// Project operations

func (s *SQLiteStore) CreateProject(name, description string) (*models.Project, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO projects (id, name, description, created_at)
		VALUES (?, ?, ?, ?)
	`, id, name, description, now)
	if err != nil {
		// Check for unique constraint violation
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return &models.Project{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   now,
	}, nil
}

func (s *SQLiteStore) GetProject(id string) (*models.Project, error) {
	var p models.Project
	var desc sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, description, created_at FROM projects WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &desc, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	p.Description = desc.String
	return &p, nil
}

func (s *SQLiteStore) GetProjectByName(name string) (*models.Project, error) {
	var p models.Project
	var desc sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, description, created_at FROM projects WHERE name = ?
	`, name).Scan(&p.ID, &p.Name, &desc, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project by name: %w", err)
	}
	p.Description = desc.String
	return &p, nil
}

func (s *SQLiteStore) ListProjects() ([]models.Project, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, created_at FROM projects ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	projects := []models.Project{}
	for rows.Next() {
		var p models.Project
		var desc sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &desc, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		p.Description = desc.String
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *SQLiteStore) DeleteProject(id string) error {
	result, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// Environment operations

func (s *SQLiteStore) CreateEnvironment(projectID, name string) (*models.Environment, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO environments (id, project_id, name, created_at)
		VALUES (?, ?, ?, ?)
	`, id, projectID, name, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	return &models.Environment{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		CreatedAt: now,
	}, nil
}

func (s *SQLiteStore) GetEnvironment(id string) (*models.Environment, error) {
	var e models.Environment
	var parentID sql.NullString
	err := s.db.QueryRow(`
		SELECT id, project_id, name, parent_id, created_at FROM environments WHERE id = ?
	`, id).Scan(&e.ID, &e.ProjectID, &e.Name, &parentID, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}
	if parentID.Valid {
		e.ParentID = &parentID.String
	}
	return &e, nil
}

func (s *SQLiteStore) GetEnvironmentByName(projectID, name string) (*models.Environment, error) {
	var e models.Environment
	var parentID sql.NullString
	err := s.db.QueryRow(`
		SELECT id, project_id, name, parent_id, created_at FROM environments
		WHERE project_id = ? AND name = ?
	`, projectID, name).Scan(&e.ID, &e.ProjectID, &e.Name, &parentID, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get environment by name: %w", err)
	}
	if parentID.Valid {
		e.ParentID = &parentID.String
	}
	return &e, nil
}

func (s *SQLiteStore) ListEnvironments(projectID string) ([]models.Environment, error) {
	rows, err := s.db.Query(`
		SELECT id, project_id, name, parent_id, created_at FROM environments
		WHERE project_id = ? ORDER BY name
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	defer rows.Close()

	envs := []models.Environment{}
	for rows.Next() {
		var e models.Environment
		var parentID sql.NullString
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &parentID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan environment: %w", err)
		}
		if parentID.Valid {
			e.ParentID = &parentID.String
		}
		envs = append(envs, e)
	}
	return envs, rows.Err()
}

func (s *SQLiteStore) DeleteEnvironment(id string) error {
	result, err := s.db.Exec(`DELETE FROM environments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateEnvironmentWithParent creates an environment with a parent for inheritance
func (s *SQLiteStore) CreateEnvironmentWithParent(projectID, name, parentID string) (*models.Environment, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO environments (id, project_id, name, parent_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, projectID, name, parentID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	return &models.Environment{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		ParentID:  &parentID,
		CreatedAt: now,
	}, nil
}

// GetEnvironmentAncestors returns the chain of parent environments (from immediate parent to root)
func (s *SQLiteStore) GetEnvironmentAncestors(envID string) ([]models.Environment, error) {
	ancestors := []models.Environment{}
	currentID := envID
	visited := make(map[string]bool)

	// Walk up the chain, max 10 levels to prevent infinite loops
	for i := 0; i < 10; i++ {
		var e models.Environment
		var parentID sql.NullString
		err := s.db.QueryRow(`
			SELECT id, project_id, name, parent_id, created_at FROM environments WHERE id = ?
		`, currentID).Scan(&e.ID, &e.ProjectID, &e.Name, &parentID, &e.CreatedAt)
		if err == sql.ErrNoRows {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get ancestor: %w", err)
		}

		if parentID.Valid {
			e.ParentID = &parentID.String
		}

		// Skip the first one (it's the environment itself, not an ancestor)
		if currentID != envID {
			// Check for circular reference
			if visited[e.ID] {
				return nil, fmt.Errorf("circular inheritance detected")
			}
			visited[e.ID] = true
			ancestors = append(ancestors, e)
		}

		// Move to parent
		if !parentID.Valid {
			break
		}
		currentID = parentID.String
	}

	return ancestors, nil
}

// GetEnvironmentChildren returns all direct children of an environment
func (s *SQLiteStore) GetEnvironmentChildren(envID string) ([]models.Environment, error) {
	rows, err := s.db.Query(`
		SELECT id, project_id, name, parent_id, created_at FROM environments
		WHERE parent_id = ? ORDER BY name
	`, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get children: %w", err)
	}
	defer rows.Close()

	children := []models.Environment{}
	for rows.Next() {
		var e models.Environment
		var parentID sql.NullString
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &parentID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan child: %w", err)
		}
		if parentID.Valid {
			e.ParentID = &parentID.String
		}
		children = append(children, e)
	}
	return children, rows.Err()
}

// GetSecretWithInheritance gets a secret, walking up the inheritance chain if not found locally
func (s *SQLiteStore) GetSecretWithInheritance(envID, key string) (*models.MergedSecret, error) {
	// First try to get the secret directly from this environment
	env, err := s.GetEnvironment(envID)
	if err != nil {
		return nil, err
	}

	sec, err := s.GetSecret(envID, key)
	if err == nil {
		// Found locally
		return &models.MergedSecret{
			Secret:        *sec,
			SourceEnvID:   envID,
			SourceEnvName: env.Name,
			IsInherited:   false,
		}, nil
	}
	if err != ErrNotFound {
		return nil, err
	}

	// Not found locally, walk up the chain
	ancestors, err := s.GetEnvironmentAncestors(envID)
	if err != nil {
		return nil, err
	}

	for _, ancestor := range ancestors {
		sec, err := s.GetSecret(ancestor.ID, key)
		if err == nil {
			return &models.MergedSecret{
				Secret:        *sec,
				SourceEnvID:   ancestor.ID,
				SourceEnvName: ancestor.Name,
				IsInherited:   true,
			}, nil
		}
		if err != ErrNotFound {
			return nil, err
		}
	}

	return nil, ErrNotFound
}

// ListSecretsWithInheritance lists all secrets including inherited ones
func (s *SQLiteStore) ListSecretsWithInheritance(envID string) ([]models.MergedSecret, error) {
	// Get the environment
	env, err := s.GetEnvironment(envID)
	if err != nil {
		return nil, err
	}

	// Build the chain: [current, parent, grandparent, ...]
	chain := []models.Environment{*env}
	ancestors, err := s.GetEnvironmentAncestors(envID)
	if err != nil {
		return nil, err
	}
	chain = append(chain, ancestors...)

	// Process from root to child (reverse order) so child overrides parent
	secretMap := make(map[string]models.MergedSecret)
	for i := len(chain) - 1; i >= 0; i-- {
		ancestor := chain[i]
		secrets, err := s.ListSecrets(ancestor.ID)
		if err != nil {
			return nil, err
		}

		for _, sec := range secrets {
			isInherited := ancestor.ID != envID
			secretMap[sec.Key] = models.MergedSecret{
				Secret:        sec,
				SourceEnvID:   ancestor.ID,
				SourceEnvName: ancestor.Name,
				IsInherited:   isInherited,
			}
		}
	}

	// Convert map to sorted slice
	result := make([]models.MergedSecret, 0, len(secretMap))
	for _, ms := range secretMap {
		result = append(result, ms)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result, nil
}

// Secret operations

func (s *SQLiteStore) CreateSecret(envID, key string, encryptedValue, nonce []byte) (*models.Secret, error) {
	id := uuid.New().String()
	now := time.Now()

	// Start transaction for secret + history
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO secrets (id, environment_id, key, encrypted_value, nonce, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
	`, id, envID, key, encryptedValue, nonce, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	// Record in history
	historyID := uuid.New().String()
	_, err = tx.Exec(`
		INSERT INTO secret_history (id, environment_id, key, encrypted_value, nonce, version, change_type, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
	`, historyID, envID, key, encryptedValue, nonce, models.ChangeTypeCreate, now)
	if err != nil {
		return nil, fmt.Errorf("failed to record history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return &models.Secret{
		ID:             id,
		EnvironmentID:  envID,
		Key:            key,
		EncryptedValue: encryptedValue,
		Nonce:          nonce,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (s *SQLiteStore) UpdateSecret(envID, key string, encryptedValue, nonce []byte) (*models.Secret, error) {
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current version
	var currentVersion int
	var id string
	err = tx.QueryRow(`
		SELECT id, version FROM secrets WHERE environment_id = ? AND key = ?
	`, envID, key).Scan(&id, &currentVersion)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get current secret: %w", err)
	}

	newVersion := currentVersion + 1

	// Update secret
	_, err = tx.Exec(`
		UPDATE secrets SET encrypted_value = ?, nonce = ?, version = ?, updated_at = ?
		WHERE environment_id = ? AND key = ?
	`, encryptedValue, nonce, newVersion, now, envID, key)
	if err != nil {
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	// Record in history
	historyID := uuid.New().String()
	_, err = tx.Exec(`
		INSERT INTO secret_history (id, environment_id, key, encrypted_value, nonce, version, change_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, historyID, envID, key, encryptedValue, nonce, newVersion, models.ChangeTypeUpdate, now)
	if err != nil {
		return nil, fmt.Errorf("failed to record history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return &models.Secret{
		ID:             id,
		EnvironmentID:  envID,
		Key:            key,
		EncryptedValue: encryptedValue,
		Nonce:          nonce,
		Version:        newVersion,
		UpdatedAt:      now,
	}, nil
}

func (s *SQLiteStore) GetSecret(envID, key string) (*models.Secret, error) {
	var sec models.Secret
	err := s.db.QueryRow(`
		SELECT id, environment_id, key, encrypted_value, nonce, version, created_at, updated_at
		FROM secrets WHERE environment_id = ? AND key = ?
	`, envID, key).Scan(&sec.ID, &sec.EnvironmentID, &sec.Key, &sec.EncryptedValue, &sec.Nonce, &sec.Version, &sec.CreatedAt, &sec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	return &sec, nil
}

func (s *SQLiteStore) ListSecrets(envID string) ([]models.Secret, error) {
	rows, err := s.db.Query(`
		SELECT id, environment_id, key, encrypted_value, nonce, version, created_at, updated_at
		FROM secrets WHERE environment_id = ? ORDER BY key
	`, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	defer rows.Close()

	secrets := []models.Secret{}
	for rows.Next() {
		var sec models.Secret
		if err := rows.Scan(&sec.ID, &sec.EnvironmentID, &sec.Key, &sec.EncryptedValue, &sec.Nonce, &sec.Version, &sec.CreatedAt, &sec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan secret: %w", err)
		}
		secrets = append(secrets, sec)
	}
	return secrets, rows.Err()
}

func (s *SQLiteStore) DeleteSecret(envID, key string) error {
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current secret for history
	var encryptedValue, nonce []byte
	var version int
	err = tx.QueryRow(`
		SELECT encrypted_value, nonce, version FROM secrets WHERE environment_id = ? AND key = ?
	`, envID, key).Scan(&encryptedValue, &nonce, &version)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get secret for deletion: %w", err)
	}

	// Delete secret
	_, err = tx.Exec(`DELETE FROM secrets WHERE environment_id = ? AND key = ?`, envID, key)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Record in history with incremented version
	historyID := uuid.New().String()
	_, err = tx.Exec(`
		INSERT INTO secret_history (id, environment_id, key, encrypted_value, nonce, version, change_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, historyID, envID, key, encryptedValue, nonce, version+1, models.ChangeTypeDelete, now)
	if err != nil {
		return fmt.Errorf("failed to record deletion history: %w", err)
	}

	return tx.Commit()
}

// Secret history operations

func (s *SQLiteStore) GetSecretHistory(envID, key string, limit int) ([]models.SecretHistory, error) {
	rows, err := s.db.Query(`
		SELECT id, environment_id, key, encrypted_value, nonce, version, change_type, created_at
		FROM secret_history WHERE environment_id = ? AND key = ?
		ORDER BY version DESC LIMIT ?
	`, envID, key, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret history: %w", err)
	}
	defer rows.Close()

	history := []models.SecretHistory{}
	for rows.Next() {
		var h models.SecretHistory
		if err := rows.Scan(&h.ID, &h.EnvironmentID, &h.Key, &h.EncryptedValue, &h.Nonce, &h.Version, &h.ChangeType, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

func (s *SQLiteStore) GetSecretVersion(envID, key string, version int) (*models.SecretHistory, error) {
	var h models.SecretHistory
	err := s.db.QueryRow(`
		SELECT id, environment_id, key, encrypted_value, nonce, version, change_type, created_at
		FROM secret_history WHERE environment_id = ? AND key = ? AND version = ?
	`, envID, key, version).Scan(&h.ID, &h.EnvironmentID, &h.Key, &h.EncryptedValue, &h.Nonce, &h.Version, &h.ChangeType, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get secret version: %w", err)
	}
	return &h, nil
}

// Config operations

func (s *SQLiteStore) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get config: %w", err)
	}
	return value, nil
}

func (s *SQLiteStore) SetConfig(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteConfig(key string) error {
	_, err := s.db.Exec(`DELETE FROM config WHERE key = ?`, key)
	return err
}

// Audit operations

func (s *SQLiteStore) LogAudit(log *models.AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	_, err := s.db.Exec(`
		INSERT INTO audit_log (id, timestamp, action, project_id, environment_id, secret_key, success, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, log.ID, log.Timestamp, log.Action, nullString(log.ProjectID), nullString(log.EnvironmentID), nullString(log.SecretKey), log.Success, nullString(log.ErrorMessage))
	if err != nil {
		return fmt.Errorf("failed to log audit: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetAuditLogs(limit int) ([]models.AuditLog, error) {
	rows, err := s.db.Query(`
		SELECT id, timestamp, action, project_id, environment_id, secret_key, success, error_message
		FROM audit_log ORDER BY timestamp DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}
	defer rows.Close()

	logs := []models.AuditLog{}
	for rows.Next() {
		var l models.AuditLog
		var projectID, envID, secretKey, errMsg sql.NullString
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.Action, &projectID, &envID, &secretKey, &l.Success, &errMsg); err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		l.ProjectID = projectID.String
		l.EnvironmentID = envID.String
		l.SecretKey = secretKey.String
		l.ErrorMessage = errMsg.String
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// Helper function for nullable strings
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
