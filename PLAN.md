# Coffer - Self-Hosted Secrets Manager

## Overview
A self-hosted secrets manager CLI that replaces Infisical. Runs locally with SQLite, with optional Litestream replication to Tigris S3 for backup/sync.

## Key Decisions
- **Language**: Go (Cobra CLI)
- **Database**: SQLite with WAL mode
- **Encryption**: AES-256-GCM with Argon2id key derivation
- **Auth**: Master password OR OS keychain (configurable)
- **Backup**: Litestream to Tigris S3

## CLI Commands

```bash
# Initialize & Auth
coffer init                           # Create vault with master password
coffer unlock                         # Unlock vault (password or keychain)
coffer lock                           # Lock vault

# Projects
coffer project create myapp
coffer project list
coffer project use myapp

# Environments
coffer env create dev|staging|prod
coffer env list

# Secrets
coffer set DATABASE_URL "postgres://..." --env prod
coffer get DATABASE_URL --env prod
coffer list --env prod
coffer delete DATABASE_URL --env prod

# The main feature - inject secrets and run command
coffer run --env prod -- npm start
coffer run --env dev -- ./my-app

# Import/Export
coffer import .env --env dev
coffer export --env prod > .env.prod

# History
coffer history DATABASE_URL --env prod
coffer restore DATABASE_URL --env prod --version 3
```

## Project Structure

```
coffer/
├── main.go
├── go.mod
├── Makefile
├── PLAN.md
├── cmd/
│   ├── root.go
│   ├── init.go
│   ├── unlock.go
│   ├── lock.go
│   ├── project.go
│   ├── env.go
│   ├── set.go
│   ├── get.go
│   ├── list.go
│   ├── delete.go
│   ├── run.go
│   ├── import.go
│   ├── export.go
│   └── history.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── crypto/
│   │   ├── crypto.go         # AES-256-GCM
│   │   ├── kdf.go            # Argon2id key derivation
│   │   ├── keychain.go       # OS keychain integration
│   │   └── crypto_test.go
│   ├── models/
│   │   └── models.go
│   ├── store/
│   │   ├── store.go          # Interface
│   │   ├── sqlite.go         # Implementation
│   │   └── store_test.go
│   ├── vault/
│   │   ├── vault.go
│   │   └── vault_test.go
│   └── resolver/
│       ├── resolver.go       # ${SECRET_REF} resolution
│       └── resolver_test.go
└── litestream.yml.example
```

## Database Schema

```sql
-- Projects
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Environments (dev, staging, prod per project)
CREATE TABLE environments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, name)
);

-- Secrets (encrypted)
CREATE TABLE secrets (
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

-- Secret history for versioning
CREATE TABLE secret_history (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL,
    key TEXT NOT NULL,
    encrypted_value BLOB NOT NULL,
    nonce BLOB NOT NULL,
    version INTEGER NOT NULL,
    change_type TEXT NOT NULL,  -- 'create', 'update', 'delete'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Vault metadata (salt, key verification)
CREATE TABLE vault_meta (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    salt BLOB NOT NULL,
    key_check BLOB NOT NULL,
    key_check_nonce BLOB NOT NULL,
    keychain_enabled BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Audit log
CREATE TABLE audit_log (
    id TEXT PRIMARY KEY,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    action TEXT NOT NULL,
    project_id TEXT,
    environment_id TEXT,
    secret_key TEXT,
    success BOOLEAN DEFAULT 1,
    error_message TEXT
);

-- Active project config
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

## Encryption Flow

1. **Init**: User provides master password
2. **Key Derivation**: `Argon2id(password, random_salt)` → 256-bit key
3. **Verification**: Encrypt known value, store for password verification
4. **Encrypt Secret**: `AES-256-GCM(key, random_nonce, plaintext, key_name_as_aad)`
5. **Session**: Derived key cached to file (encrypted with session key)

## Implementation Phases (with tests after each)

### Phase 1: Foundation
- [x] Create git repo, go.mod, Makefile
- [x] Config package (~/.coffer/ directory)
- [x] **Tests**: config_test.go

### Phase 2: Crypto
- [x] Argon2id key derivation
- [x] AES-256-GCM encrypt/decrypt
- [x] **Tests**: crypto_test.go

### Phase 3: Models & Store
- [x] Models package
- [x] Store interface
- [x] SQLite implementation with schema
- [x] **Tests**: store_test.go

### Phase 4: Vault Management
- [x] Vault init/unlock/lock logic
- [x] Session management
- [x] **Tests**: vault_test.go

### Phase 5: CLI - Init/Unlock/Lock
- [x] Cobra root command
- [x] init command
- [x] unlock command
- [x] lock command

### Phase 6: CLI - Projects & Environments
- [x] project create/list/use/delete
- [x] env create/list/delete

### Phase 7: CLI - Secrets
- [x] set command
- [x] get command
- [x] list command
- [x] delete command

### Phase 8: Secret Resolution
- [x] Resolver for ${SECRET_REF} syntax
- [x] **Tests**: resolver_test.go

### Phase 9: CLI - Run Command
- [x] coffer run --env <env> -- <command>
- [x] Inject secrets as environment variables

### Phase 10: Import/Export
- [x] import from .env files
- [x] export to .env format
- [x] export to JSON format

### Phase 11: History
- [x] history command
- [x] restore command

### Phase 12: Keychain Integration
- [x] OS keychain support via go-keyring
- [x] keychain enable/disable commands
- [x] Auto-use keychain on unlock if enabled

### Phase 13: Final Testing & Polish
- [x] Litestream config example
- [ ] End-to-end CLI tests (optional)

### Phase 14: Environment Branching & Inheritance
- [x] Schema migration: Add parent_id column to environments table
- [x] Model changes: Add ParentID to Environment, add MergedSecret struct
- [x] Store interface: Add new inheritance methods
- [x] SQLite implementation: Implement inheritance queries
- [x] CLI: Add `coffer env branch` command
- [x] CLI: Update `env list` to show hierarchy
- [x] CLI: Block parent deletion if has children
- [x] CLI: Update get, list, run, export to use inheritance
- [x] Tests: Add inheritance tests (3 new tests)
- [x] Docs: Update README with branching

## Status: COMPLETE (Core Features + Environment Branching)

All core features implemented and tested:
- 51 unit tests passing
- Full CLI with all commands working
- Secret encryption/decryption
- Multi-project/environment support
- Secret versioning and history
- Import/export functionality
- Secret reference resolution

## Dependencies

```go
require (
    github.com/spf13/cobra v1.8.0
    github.com/google/uuid v1.6.0
    github.com/zalando/go-keyring v0.2.4
    golang.org/x/crypto v0.28.0
    golang.org/x/term v0.25.0
    modernc.org/sqlite v1.33.0
)
```

## Security Notes
- Secrets encrypted at rest with AES-256-GCM
- Key derived with Argon2id (memory-hard, resistant to GPU attacks)
- Nonce is random per encryption (never reused)
- AAD includes key name (prevents value swapping)
- Audit log tracks all access (never logs values)
- File permissions: 0600 for db, 0700 for directory
