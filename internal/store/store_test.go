package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/russellromney/coffer/internal/models"
)

func setupTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func TestNewSQLiteStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestVaultMeta(t *testing.T) {
	store := setupTestStore(t)

	// Initially should not exist
	_, err := store.GetVaultMeta()
	if err != ErrNotFound {
		t.Errorf("GetVaultMeta() error = %v, want ErrNotFound", err)
	}

	// Create vault meta
	salt := []byte("test-salt-16byte")
	keyCheck := []byte("encrypted-check")
	keyCheckNonce := []byte("12-byte-nonc")

	err = store.CreateVaultMeta(salt, keyCheck, keyCheckNonce)
	if err != nil {
		t.Fatalf("CreateVaultMeta() error = %v", err)
	}

	// Get vault meta
	meta, err := store.GetVaultMeta()
	if err != nil {
		t.Fatalf("GetVaultMeta() error = %v", err)
	}

	if string(meta.Salt) != string(salt) {
		t.Errorf("VaultMeta.Salt = %v, want %v", meta.Salt, salt)
	}
	if string(meta.KeyCheck) != string(keyCheck) {
		t.Errorf("VaultMeta.KeyCheck = %v, want %v", meta.KeyCheck, keyCheck)
	}
	if meta.KeychainEnabled {
		t.Error("VaultMeta.KeychainEnabled should be false by default")
	}

	// Update keychain enabled
	err = store.SetKeychainEnabled(true)
	if err != nil {
		t.Fatalf("SetKeychainEnabled() error = %v", err)
	}

	meta, _ = store.GetVaultMeta()
	if !meta.KeychainEnabled {
		t.Error("VaultMeta.KeychainEnabled should be true after update")
	}
}

func TestProjects(t *testing.T) {
	store := setupTestStore(t)

	// Create project
	project, err := store.CreateProject("myapp", "My Application")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	if project.ID == "" {
		t.Error("Project.ID is empty")
	}
	if project.Name != "myapp" {
		t.Errorf("Project.Name = %v, want myapp", project.Name)
	}
	if project.Description != "My Application" {
		t.Errorf("Project.Description = %v, want 'My Application'", project.Description)
	}

	// Get project by ID
	got, err := store.GetProject(project.ID)
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if got.Name != project.Name {
		t.Errorf("GetProject().Name = %v, want %v", got.Name, project.Name)
	}

	// Get project by name
	got, err = store.GetProjectByName("myapp")
	if err != nil {
		t.Fatalf("GetProjectByName() error = %v", err)
	}
	if got.ID != project.ID {
		t.Errorf("GetProjectByName().ID = %v, want %v", got.ID, project.ID)
	}

	// List projects
	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("ListProjects() count = %d, want 1", len(projects))
	}

	// Create another project
	_, err = store.CreateProject("otherapp", "")
	if err != nil {
		t.Fatalf("CreateProject() second error = %v", err)
	}

	projects, _ = store.ListProjects()
	if len(projects) != 2 {
		t.Errorf("ListProjects() count = %d, want 2", len(projects))
	}

	// Delete project
	err = store.DeleteProject(project.ID)
	if err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}

	// Should not exist anymore
	_, err = store.GetProject(project.ID)
	if err != ErrNotFound {
		t.Errorf("GetProject() after delete error = %v, want ErrNotFound", err)
	}

	// Delete non-existent project
	err = store.DeleteProject("non-existent-id")
	if err != ErrNotFound {
		t.Errorf("DeleteProject() non-existent error = %v, want ErrNotFound", err)
	}
}

func TestEnvironments(t *testing.T) {
	store := setupTestStore(t)

	// Create project first
	project, _ := store.CreateProject("myapp", "")

	// Create environment
	env, err := store.CreateEnvironment(project.ID, "dev")
	if err != nil {
		t.Fatalf("CreateEnvironment() error = %v", err)
	}

	if env.ID == "" {
		t.Error("Environment.ID is empty")
	}
	if env.ProjectID != project.ID {
		t.Errorf("Environment.ProjectID = %v, want %v", env.ProjectID, project.ID)
	}
	if env.Name != "dev" {
		t.Errorf("Environment.Name = %v, want dev", env.Name)
	}

	// Get environment by ID
	got, err := store.GetEnvironment(env.ID)
	if err != nil {
		t.Fatalf("GetEnvironment() error = %v", err)
	}
	if got.Name != env.Name {
		t.Errorf("GetEnvironment().Name = %v, want %v", got.Name, env.Name)
	}

	// Get environment by name
	got, err = store.GetEnvironmentByName(project.ID, "dev")
	if err != nil {
		t.Fatalf("GetEnvironmentByName() error = %v", err)
	}
	if got.ID != env.ID {
		t.Errorf("GetEnvironmentByName().ID = %v, want %v", got.ID, env.ID)
	}

	// Create more environments
	store.CreateEnvironment(project.ID, "staging")
	store.CreateEnvironment(project.ID, "prod")

	// List environments
	envs, err := store.ListEnvironments(project.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}
	if len(envs) != 3 {
		t.Errorf("ListEnvironments() count = %d, want 3", len(envs))
	}

	// Delete environment
	err = store.DeleteEnvironment(env.ID)
	if err != nil {
		t.Fatalf("DeleteEnvironment() error = %v", err)
	}

	_, err = store.GetEnvironment(env.ID)
	if err != ErrNotFound {
		t.Errorf("GetEnvironment() after delete error = %v, want ErrNotFound", err)
	}
}

func TestEnvironmentCascadeDelete(t *testing.T) {
	store := setupTestStore(t)

	project, _ := store.CreateProject("myapp", "")
	env, _ := store.CreateEnvironment(project.ID, "dev")

	// Create a secret in the environment
	store.CreateSecret(env.ID, "API_KEY", []byte("encrypted"), []byte("nonce123456"))

	// Delete project should cascade delete environments and secrets
	err := store.DeleteProject(project.ID)
	if err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}

	// Environment should be gone
	_, err = store.GetEnvironment(env.ID)
	if err != ErrNotFound {
		t.Errorf("GetEnvironment() after cascade delete error = %v, want ErrNotFound", err)
	}
}

func TestSecrets(t *testing.T) {
	store := setupTestStore(t)

	project, _ := store.CreateProject("myapp", "")
	env, _ := store.CreateEnvironment(project.ID, "dev")

	// Create secret
	encryptedValue := []byte("encrypted-secret-value")
	nonce := []byte("12-byte-nonc")

	secret, err := store.CreateSecret(env.ID, "DATABASE_URL", encryptedValue, nonce)
	if err != nil {
		t.Fatalf("CreateSecret() error = %v", err)
	}

	if secret.ID == "" {
		t.Error("Secret.ID is empty")
	}
	if secret.Key != "DATABASE_URL" {
		t.Errorf("Secret.Key = %v, want DATABASE_URL", secret.Key)
	}
	if secret.Version != 1 {
		t.Errorf("Secret.Version = %d, want 1", secret.Version)
	}
	if string(secret.EncryptedValue) != string(encryptedValue) {
		t.Error("Secret.EncryptedValue mismatch")
	}

	// Get secret
	got, err := store.GetSecret(env.ID, "DATABASE_URL")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if got.ID != secret.ID {
		t.Errorf("GetSecret().ID = %v, want %v", got.ID, secret.ID)
	}

	// Update secret
	newEncrypted := []byte("new-encrypted-value")
	newNonce := []byte("new-nonce123")

	updated, err := store.UpdateSecret(env.ID, "DATABASE_URL", newEncrypted, newNonce)
	if err != nil {
		t.Fatalf("UpdateSecret() error = %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("UpdateSecret().Version = %d, want 2", updated.Version)
	}
	if string(updated.EncryptedValue) != string(newEncrypted) {
		t.Error("UpdateSecret().EncryptedValue mismatch")
	}

	// List secrets
	secrets, err := store.ListSecrets(env.ID)
	if err != nil {
		t.Fatalf("ListSecrets() error = %v", err)
	}
	if len(secrets) != 1 {
		t.Errorf("ListSecrets() count = %d, want 1", len(secrets))
	}

	// Create more secrets
	store.CreateSecret(env.ID, "API_KEY", []byte("api"), []byte("nonce123456"))
	store.CreateSecret(env.ID, "JWT_SECRET", []byte("jwt"), []byte("nonce123456"))

	secrets, _ = store.ListSecrets(env.ID)
	if len(secrets) != 3 {
		t.Errorf("ListSecrets() count = %d, want 3", len(secrets))
	}

	// Delete secret
	err = store.DeleteSecret(env.ID, "DATABASE_URL")
	if err != nil {
		t.Fatalf("DeleteSecret() error = %v", err)
	}

	_, err = store.GetSecret(env.ID, "DATABASE_URL")
	if err != ErrNotFound {
		t.Errorf("GetSecret() after delete error = %v, want ErrNotFound", err)
	}

	// Update non-existent secret
	_, err = store.UpdateSecret(env.ID, "NON_EXISTENT", []byte("value"), []byte("nonce123456"))
	if err != ErrNotFound {
		t.Errorf("UpdateSecret() non-existent error = %v, want ErrNotFound", err)
	}
}

func TestSecretHistory(t *testing.T) {
	store := setupTestStore(t)

	project, _ := store.CreateProject("myapp", "")
	env, _ := store.CreateEnvironment(project.ID, "dev")

	// Create secret (version 1)
	store.CreateSecret(env.ID, "API_KEY", []byte("value1"), []byte("nonce123456"))

	// Update secret (version 2)
	store.UpdateSecret(env.ID, "API_KEY", []byte("value2"), []byte("nonce789012"))

	// Update secret (version 3)
	store.UpdateSecret(env.ID, "API_KEY", []byte("value3"), []byte("nonce345678"))

	// Get history
	history, err := store.GetSecretHistory(env.ID, "API_KEY", 10)
	if err != nil {
		t.Fatalf("GetSecretHistory() error = %v", err)
	}
	if len(history) != 3 {
		t.Errorf("GetSecretHistory() count = %d, want 3", len(history))
	}

	// History should be in descending order (newest first)
	if history[0].Version != 3 {
		t.Errorf("History[0].Version = %d, want 3", history[0].Version)
	}
	if history[1].Version != 2 {
		t.Errorf("History[1].Version = %d, want 2", history[1].Version)
	}
	if history[2].Version != 1 {
		t.Errorf("History[2].Version = %d, want 1", history[2].Version)
	}

	// Check change types
	if history[0].ChangeType != models.ChangeTypeUpdate {
		t.Errorf("History[0].ChangeType = %v, want update", history[0].ChangeType)
	}
	if history[2].ChangeType != models.ChangeTypeCreate {
		t.Errorf("History[2].ChangeType = %v, want create", history[2].ChangeType)
	}

	// Get specific version
	v2, err := store.GetSecretVersion(env.ID, "API_KEY", 2)
	if err != nil {
		t.Fatalf("GetSecretVersion() error = %v", err)
	}
	if string(v2.EncryptedValue) != "value2" {
		t.Errorf("GetSecretVersion(2).EncryptedValue = %v, want value2", string(v2.EncryptedValue))
	}

	// Delete secret should record in history with version 4
	store.DeleteSecret(env.ID, "API_KEY")

	history, _ = store.GetSecretHistory(env.ID, "API_KEY", 10)
	if len(history) != 4 {
		t.Errorf("GetSecretHistory() after delete count = %d, want 4", len(history))
	}
	if history[0].ChangeType != models.ChangeTypeDelete {
		t.Errorf("History[0].ChangeType after delete = %v, want delete", history[0].ChangeType)
	}
	if history[0].Version != 4 {
		t.Errorf("History[0].Version after delete = %d, want 4", history[0].Version)
	}
}

func TestConfig(t *testing.T) {
	store := setupTestStore(t)

	// Get non-existent config
	_, err := store.GetConfig("active_project")
	if err != ErrNotFound {
		t.Errorf("GetConfig() non-existent error = %v, want ErrNotFound", err)
	}

	// Set config
	err = store.SetConfig("active_project", "project-123")
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	// Get config
	value, err := store.GetConfig("active_project")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if value != "project-123" {
		t.Errorf("GetConfig() = %v, want project-123", value)
	}

	// Update config (upsert)
	err = store.SetConfig("active_project", "project-456")
	if err != nil {
		t.Fatalf("SetConfig() update error = %v", err)
	}

	value, _ = store.GetConfig("active_project")
	if value != "project-456" {
		t.Errorf("GetConfig() after update = %v, want project-456", value)
	}

	// Delete config
	err = store.DeleteConfig("active_project")
	if err != nil {
		t.Fatalf("DeleteConfig() error = %v", err)
	}

	_, err = store.GetConfig("active_project")
	if err != ErrNotFound {
		t.Errorf("GetConfig() after delete error = %v, want ErrNotFound", err)
	}
}

func TestAuditLog(t *testing.T) {
	store := setupTestStore(t)

	// Log audit entry
	log := &models.AuditLog{
		Action:        models.ActionRead,
		ProjectID:     "proj-123",
		EnvironmentID: "env-456",
		SecretKey:     "API_KEY",
		Success:       true,
	}

	err := store.LogAudit(log)
	if err != nil {
		t.Fatalf("LogAudit() error = %v", err)
	}

	if log.ID == "" {
		t.Error("AuditLog.ID should be set")
	}

	// Log more entries
	store.LogAudit(&models.AuditLog{
		Action:       models.ActionCreate,
		SecretKey:    "DATABASE_URL",
		Success:      true,
	})
	store.LogAudit(&models.AuditLog{
		Action:       models.ActionDelete,
		SecretKey:    "OLD_KEY",
		Success:      false,
		ErrorMessage: "permission denied",
	})

	// Get logs
	logs, err := store.GetAuditLogs(10)
	if err != nil {
		t.Fatalf("GetAuditLogs() error = %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("GetAuditLogs() count = %d, want 3", len(logs))
	}

	// Should be in descending order (newest first)
	if logs[0].Action != models.ActionDelete {
		t.Errorf("Logs[0].Action = %v, want delete", logs[0].Action)
	}

	// Limit should work
	logs, _ = store.GetAuditLogs(1)
	if len(logs) != 1 {
		t.Errorf("GetAuditLogs(1) count = %d, want 1", len(logs))
	}
}

func TestEnvironmentInheritance(t *testing.T) {
	store := setupTestStore(t)

	project, _ := store.CreateProject("myapp", "")

	// Create root environment with secrets
	dev, _ := store.CreateEnvironment(project.ID, "dev")
	store.CreateSecret(dev.ID, "DATABASE_URL", []byte("dev-db"), []byte("nonce123456"))
	store.CreateSecret(dev.ID, "API_KEY", []byte("dev-api"), []byte("nonce123456"))
	store.CreateSecret(dev.ID, "LOG_LEVEL", []byte("debug"), []byte("nonce123456"))

	// Create branch environment
	devPersonal, err := store.CreateEnvironmentWithParent(project.ID, "dev_personal", dev.ID)
	if err != nil {
		t.Fatalf("CreateEnvironmentWithParent() error = %v", err)
	}
	if devPersonal.ParentID == nil || *devPersonal.ParentID != dev.ID {
		t.Error("Branch environment should have parent ID set")
	}

	// Override DATABASE_URL in branch
	store.CreateSecret(devPersonal.ID, "DATABASE_URL", []byte("personal-db"), []byte("nonce123456"))

	// Test GetEnvironmentAncestors
	ancestors, err := store.GetEnvironmentAncestors(devPersonal.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentAncestors() error = %v", err)
	}
	if len(ancestors) != 1 {
		t.Errorf("GetEnvironmentAncestors() count = %d, want 1", len(ancestors))
	}
	if ancestors[0].ID != dev.ID {
		t.Error("GetEnvironmentAncestors() should return parent")
	}

	// Test GetEnvironmentChildren
	children, err := store.GetEnvironmentChildren(dev.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentChildren() error = %v", err)
	}
	if len(children) != 1 {
		t.Errorf("GetEnvironmentChildren() count = %d, want 1", len(children))
	}
	if children[0].ID != devPersonal.ID {
		t.Error("GetEnvironmentChildren() should return child")
	}

	// Test GetSecretWithInheritance - local secret
	merged, err := store.GetSecretWithInheritance(devPersonal.ID, "DATABASE_URL")
	if err != nil {
		t.Fatalf("GetSecretWithInheritance(DATABASE_URL) error = %v", err)
	}
	if merged.IsInherited {
		t.Error("DATABASE_URL should NOT be inherited (it's overridden locally)")
	}
	if string(merged.EncryptedValue) != "personal-db" {
		t.Error("DATABASE_URL should be the local override value")
	}

	// Test GetSecretWithInheritance - inherited secret
	merged, err = store.GetSecretWithInheritance(devPersonal.ID, "API_KEY")
	if err != nil {
		t.Fatalf("GetSecretWithInheritance(API_KEY) error = %v", err)
	}
	if !merged.IsInherited {
		t.Error("API_KEY should be inherited")
	}
	if merged.SourceEnvName != "dev" {
		t.Errorf("API_KEY SourceEnvName = %v, want dev", merged.SourceEnvName)
	}

	// Test GetSecretWithInheritance - non-existent
	_, err = store.GetSecretWithInheritance(devPersonal.ID, "NON_EXISTENT")
	if err != ErrNotFound {
		t.Errorf("GetSecretWithInheritance(NON_EXISTENT) error = %v, want ErrNotFound", err)
	}

	// Test ListSecretsWithInheritance
	secrets, err := store.ListSecretsWithInheritance(devPersonal.ID)
	if err != nil {
		t.Fatalf("ListSecretsWithInheritance() error = %v", err)
	}
	if len(secrets) != 3 {
		t.Errorf("ListSecretsWithInheritance() count = %d, want 3", len(secrets))
	}

	// Check inheritance markers
	secretMap := make(map[string]bool) // key -> isInherited
	for _, s := range secrets {
		secretMap[s.Key] = s.IsInherited
	}

	if secretMap["DATABASE_URL"] != false {
		t.Error("DATABASE_URL should NOT be inherited")
	}
	if secretMap["API_KEY"] != true {
		t.Error("API_KEY should be inherited")
	}
	if secretMap["LOG_LEVEL"] != true {
		t.Error("LOG_LEVEL should be inherited")
	}
}

func TestMultiLevelInheritance(t *testing.T) {
	store := setupTestStore(t)

	project, _ := store.CreateProject("myapp", "")

	// Create chain: root -> level1 -> level2
	root, _ := store.CreateEnvironment(project.ID, "root")
	store.CreateSecret(root.ID, "ROOT_SECRET", []byte("root-val"), []byte("nonce123456"))
	store.CreateSecret(root.ID, "SHARED", []byte("root-shared"), []byte("nonce123456"))

	level1, _ := store.CreateEnvironmentWithParent(project.ID, "level1", root.ID)
	store.CreateSecret(level1.ID, "LEVEL1_SECRET", []byte("l1-val"), []byte("nonce123456"))
	store.CreateSecret(level1.ID, "SHARED", []byte("l1-shared"), []byte("nonce123456")) // Override

	level2, _ := store.CreateEnvironmentWithParent(project.ID, "level2", level1.ID)
	store.CreateSecret(level2.ID, "LEVEL2_SECRET", []byte("l2-val"), []byte("nonce123456"))

	// Test ancestors from level2
	ancestors, err := store.GetEnvironmentAncestors(level2.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentAncestors() error = %v", err)
	}
	if len(ancestors) != 2 {
		t.Errorf("GetEnvironmentAncestors() count = %d, want 2", len(ancestors))
	}
	// First ancestor should be immediate parent
	if ancestors[0].ID != level1.ID {
		t.Error("First ancestor should be level1")
	}
	if ancestors[1].ID != root.ID {
		t.Error("Second ancestor should be root")
	}

	// List all secrets from level2
	secrets, err := store.ListSecretsWithInheritance(level2.ID)
	if err != nil {
		t.Fatalf("ListSecretsWithInheritance() error = %v", err)
	}
	if len(secrets) != 4 {
		t.Errorf("ListSecretsWithInheritance() count = %d, want 4", len(secrets))
	}

	// Verify SHARED was overridden by level1 (not root)
	for _, s := range secrets {
		if s.Key == "SHARED" {
			if s.SourceEnvName != "level1" {
				t.Errorf("SHARED SourceEnvName = %v, want level1", s.SourceEnvName)
			}
			if string(s.EncryptedValue) != "l1-shared" {
				t.Error("SHARED should have level1's value")
			}
		}
	}
}

func TestEnvironmentParentInListEnvironments(t *testing.T) {
	store := setupTestStore(t)

	project, _ := store.CreateProject("myapp", "")
	dev, _ := store.CreateEnvironment(project.ID, "dev")
	store.CreateEnvironmentWithParent(project.ID, "dev_personal", dev.ID)

	envs, err := store.ListEnvironments(project.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}

	// Check that ParentID is populated correctly
	for _, e := range envs {
		if e.Name == "dev" {
			if e.ParentID != nil {
				t.Error("dev should have nil ParentID")
			}
		}
		if e.Name == "dev_personal" {
			if e.ParentID == nil || *e.ParentID != dev.ID {
				t.Error("dev_personal should have dev as parent")
			}
		}
	}
}

func TestEmptyLists(t *testing.T) {
	store := setupTestStore(t)

	// Empty projects
	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if projects == nil {
		t.Error("ListProjects() should return empty slice, not nil")
	}
	if len(projects) != 0 {
		t.Errorf("ListProjects() count = %d, want 0", len(projects))
	}

	// Create project for environment test
	project, _ := store.CreateProject("test", "")

	// Empty environments
	envs, err := store.ListEnvironments(project.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}
	if len(envs) != 0 {
		t.Errorf("ListEnvironments() count = %d, want 0", len(envs))
	}

	// Create environment for secrets test
	env, _ := store.CreateEnvironment(project.ID, "dev")

	// Empty secrets
	secrets, err := store.ListSecrets(env.ID)
	if err != nil {
		t.Fatalf("ListSecrets() error = %v", err)
	}
	if len(secrets) != 0 {
		t.Errorf("ListSecrets() count = %d, want 0", len(secrets))
	}

	// Empty audit logs
	logs, err := store.GetAuditLogs(10)
	if err != nil {
		t.Fatalf("GetAuditLogs() error = %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("GetAuditLogs() count = %d, want 0", len(logs))
	}
}
