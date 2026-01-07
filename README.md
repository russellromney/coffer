# Coffer

A self-hosted secrets manager CLI. Store encrypted secrets locally, inject them into your apps at runtime.

**Why Coffer?** Tired of SaaS secrets managers with plan limits? Coffer gives you the same workflow locally with SQLite, optional cloud backup via Litestream, and zero monthly fees.

## Features

- **Encrypted at rest** - AES-256-GCM encryption with Argon2id key derivation
- **Multi-project/environment** - Organize secrets by project and environment (dev/staging/prod)
- **Environment branching** - Create child environments that inherit secrets from parents
- **Secret injection** - Run any command with secrets as environment variables
- **Version history** - Track changes and restore previous versions
- **Import/Export** - Migrate from .env files, export to .env or JSON
- **OS Keychain** - Optional passwordless unlock via macOS Keychain, Windows Credential Manager, or Linux Secret Service
- **Cloud backup** - Optional Litestream replication to S3-compatible storage (Tigris, AWS, etc.)

## Installation

### From Source

```bash
git clone https://github.com/russellromney/coffer.git
cd coffer
make build
sudo mv coffer /usr/local/bin/
```

### Go Install

```bash
go install github.com/russellromney/coffer@latest
```

## Quick Start

```bash
# Initialize vault with master password
coffer init

# Create a project and environment
coffer project create myapp
coffer project use myapp
coffer env create dev

# Add secrets
coffer set DATABASE_URL "postgres://localhost:5432/myapp" --env dev
coffer set API_KEY "sk-secret-key" --env dev

# Run your app with secrets injected
coffer run --env dev -- npm start
```

## Commands

### Vault Management

```bash
coffer init                    # Create vault with master password
coffer init --password secret  # Non-interactive init
coffer unlock                  # Unlock vault (uses keychain if enabled)
coffer unlock --password pwd   # Non-interactive unlock
coffer lock                    # Lock vault
coffer status                  # Show vault status
```

### Projects

```bash
coffer project create myapp           # Create project
coffer project list                   # List all projects
coffer project use myapp              # Set active project
coffer project delete myapp           # Delete project (and all secrets)
```

### Environments

```bash
coffer env create dev                 # Create environment
coffer env create staging
coffer env create prod
coffer env list                       # List environments
coffer env delete staging             # Delete environment
```

### Environment Branching

Create child environments that inherit secrets from a parent. Useful for personal dev configs or feature branches:

```bash
# Create a branch environment inheriting from dev
coffer env branch dev dev_personal

# The child inherits all parent secrets
coffer list --env dev_personal
# Output:
# Secrets in myapp/dev_personal:
#   API_KEY [inherited from dev]
#   DATABASE_URL [inherited from dev]

# Override a secret locally
coffer set DATABASE_URL "postgres://localhost/personal" --env dev_personal

# Now DATABASE_URL is local, others still inherited
coffer list --env dev_personal
# Output:
# Secrets in myapp/dev_personal:
#   API_KEY [inherited from dev]
#   DATABASE_URL

# Running uses merged secrets (local overrides + inherited)
coffer run --env dev_personal -- npm start
```

Inheritance features:
- Child environments inherit all secrets from their parent
- Set a secret in a child to override the inherited value
- Supports multi-level inheritance (grandparent -> parent -> child)
- `coffer env list` shows inheritance relationships
- Cannot delete a parent environment that has children

### Secrets

```bash
coffer set KEY "value" --env dev      # Create or update secret
coffer get KEY --env dev              # Get secret value
coffer list --env dev                 # List all secrets in environment
coffer delete KEY --env dev           # Delete secret
```

### Secret Injection

The main feature - run any command with secrets injected as environment variables:

```bash
coffer run --env dev -- npm start
coffer run --env prod -- ./my-binary
coffer run --env dev -- docker-compose up
coffer run --env staging -- python manage.py runserver
```

### Import/Export

```bash
# Import from .env file
coffer import .env --env dev
coffer import production.env --env prod

# Export to stdout
coffer export --env dev              # .env format (default)
coffer export --env dev --format json

# Export to file
coffer export --env prod > .env.prod
```

### History & Restore

```bash
# View secret history
coffer history DATABASE_URL --env dev

# Restore to previous version
coffer restore DATABASE_URL --env dev --version 2
```

### Keychain Integration

Enable passwordless unlock using your OS keychain:

```bash
coffer keychain status                # Check keychain availability
coffer keychain enable                # Store key in keychain (prompts for password)
coffer keychain disable               # Remove key from keychain
```

Once enabled, `coffer unlock` will use the keychain automatically.

## Secret References

Secrets can reference other secrets using `${VAR}` syntax:

```bash
coffer set DB_HOST "localhost" --env dev
coffer set DB_PORT "5432" --env dev
coffer set DB_NAME "myapp" --env dev
coffer set DATABASE_URL "postgres://${DB_HOST}:${DB_PORT}/${DB_NAME}" --env dev

# When retrieved, DATABASE_URL resolves to: postgres://localhost:5432/myapp
coffer get DATABASE_URL --env dev
# Output: postgres://localhost:5432/myapp
```

## Cloud Backup with Litestream

Coffer stores everything in SQLite at `~/.coffer/vault.db`. Use [Litestream](https://litestream.io) for continuous replication to S3-compatible storage.

### Setup with Tigris (Fly.io)

1. Create a Tigris bucket:

```bash
fly storage create --name coffer-backup
```

2. Create `~/.coffer/litestream.yml`:

```yaml
dbs:
  - path: ~/.coffer/vault.db
    replicas:
      - type: s3
        bucket: coffer-backup
        path: vault
        endpoint: https://fly.storage.tigris.dev
        region: auto
        access-key-id: YOUR_ACCESS_KEY
        secret-access-key: YOUR_SECRET_KEY
        sync-interval: 60s
        snapshot-interval: 1h
        retention: 168h
```

3. Run Litestream:

```bash
# Foreground
litestream replicate -config ~/.coffer/litestream.yml

# Or add to your Procfile for overmind/foreman
# Procfile:
# litestream: litestream replicate -config ~/.coffer/litestream.yml
```

4. Restore from backup (on a new machine):

```bash
litestream restore -config ~/.coffer/litestream.yml ~/.coffer/vault.db
```

## Security

### Encryption

- **Algorithm**: AES-256-GCM (authenticated encryption)
- **Key derivation**: Argon2id with random salt (memory-hard, GPU-resistant)
- **Nonces**: Random 12-byte nonce per encryption (never reused)
- **AAD**: Secret key name used as additional authenticated data (prevents value swapping)

### Storage

- Database file: `~/.coffer/vault.db` (mode 0600)
- Data directory: `~/.coffer/` (mode 0700)
- Session file: `~/.coffer/session` (mode 0600, 8-hour expiry)

### Audit Log

All secret access is logged (without values):

```sql
SELECT timestamp, action, secret_key FROM audit_log ORDER BY timestamp DESC;
```

## Data Location

All data is stored in `~/.coffer/`:

```
~/.coffer/
├── vault.db           # SQLite database (encrypted secrets)
├── vault.db-wal       # WAL file
├── vault.db-shm       # Shared memory file
├── session            # Session token (temporary)
└── litestream.yml     # Optional backup config
```

## Development

```bash
# Build
make build

# Run tests
make test

# Clean
make clean
```

## License

MIT

## Contributing

Contributions welcome! Please open an issue first to discuss what you'd like to change.
