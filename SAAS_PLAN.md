# Coffer SaaS Plan

A hosted version of Coffer for teams who want the same workflow without self-hosting.

## Pricing Model

```
Free tier:    Up to 5 users, unlimited everything
Paid tier:    $5/month base + $1/user/month after 5 users

Examples:
- 3 users  → $0/month (free tier)
- 5 users  → $0/month (free tier)
- 10 users → $5 + $5 = $10/month
- 50 users → $5 + $45 = $50/month
```

Compare to Infisical at $18/user/month: 50 users = $900/month vs our $50/month.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (SvelteKit)                     │
│                     Cloudflare Pages / Vercel                    │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                         API Server (Rust)                        │
│                           Fly.io                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │    Auth     │  │   Secrets   │  │      Teams/Orgs         │  │
│  │  (Clerk)    │  │    CRUD     │  │  (invites, roles)       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Database Layer                           │
│  ┌─────────────────────┐  ┌─────────────────────────────────┐   │
│  │   Turso (SQLite)    │  │      Tigris S3 (Backups)        │   │
│  │  - Users/orgs       │  │  - Encrypted vault backups      │   │
│  │  - Teams            │  │  - Per-org isolation            │   │
│  │  - Encrypted secrets│  │                                 │   │
│  └─────────────────────┘  └─────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Tech Stack

| Component | Technology | Why |
|-----------|------------|-----|
| Frontend | SvelteKit | Fast, minimal, good DX |
| API | Rust (Axum) | Performance, security, reuse crypto code |
| Auth | Clerk | Easy team/org management, OAuth |
| Database | Turso | SQLite at the edge, familiar schema |
| Storage | Tigris S3 | Backup storage, cheap |
| Hosting | Fly.io | Already using it, good for Rust |
| Payments | Stripe | Standard, handles metered billing |

## Database Schema (Turso)

```sql
-- Organizations (teams)
CREATE TABLE organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    owner_id TEXT NOT NULL,  -- Clerk user ID
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT,
    plan TEXT DEFAULT 'free',  -- 'free' or 'paid'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Organization members
CREATE TABLE org_members (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES organizations(id),
    user_id TEXT NOT NULL,  -- Clerk user ID
    role TEXT NOT NULL DEFAULT 'member',  -- 'owner', 'admin', 'member'
    invited_by TEXT,
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, user_id)
);

-- Projects (per org)
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, name)
);

-- Environments (per project)
CREATE TABLE environments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    parent_id TEXT REFERENCES environments(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, name)
);

-- Secrets (encrypted, per environment)
CREATE TABLE secrets (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL REFERENCES environments(id),
    key TEXT NOT NULL,
    encrypted_value BLOB NOT NULL,
    nonce BLOB NOT NULL,
    version INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(environment_id, key)
);

-- Secret history (for versioning)
CREATE TABLE secret_history (
    id TEXT PRIMARY KEY,
    secret_id TEXT NOT NULL REFERENCES secrets(id),
    encrypted_value BLOB NOT NULL,
    nonce BLOB NOT NULL,
    version INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Audit log
CREATE TABLE audit_log (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES organizations(id),
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,  -- 'create', 'read', 'update', 'delete'
    resource_type TEXT NOT NULL,  -- 'secret', 'project', 'environment'
    resource_id TEXT,
    metadata TEXT,  -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Invitations
CREATE TABLE invitations (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES organizations(id),
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    invited_by TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL,
    accepted_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## API Endpoints

### Auth (via Clerk webhooks)
```
POST /webhooks/clerk           # User created/updated/deleted
```

### Organizations
```
GET    /api/orgs               # List user's orgs
POST   /api/orgs               # Create org
GET    /api/orgs/:slug         # Get org
PATCH  /api/orgs/:slug         # Update org
DELETE /api/orgs/:slug         # Delete org

GET    /api/orgs/:slug/members     # List members
POST   /api/orgs/:slug/members     # Invite member
DELETE /api/orgs/:slug/members/:id # Remove member
PATCH  /api/orgs/:slug/members/:id # Update role
```

### Projects
```
GET    /api/orgs/:slug/projects           # List projects
POST   /api/orgs/:slug/projects           # Create project
GET    /api/orgs/:slug/projects/:name     # Get project
DELETE /api/orgs/:slug/projects/:name     # Delete project
```

### Environments
```
GET    /api/orgs/:slug/projects/:name/envs           # List envs
POST   /api/orgs/:slug/projects/:name/envs           # Create env
POST   /api/orgs/:slug/projects/:name/envs/branch    # Branch env
DELETE /api/orgs/:slug/projects/:name/envs/:env      # Delete env
```

### Secrets
```
GET    /api/orgs/:slug/projects/:name/envs/:env/secrets           # List secrets
POST   /api/orgs/:slug/projects/:name/envs/:env/secrets           # Set secret
GET    /api/orgs/:slug/projects/:name/envs/:env/secrets/:key      # Get secret
DELETE /api/orgs/:slug/projects/:name/envs/:env/secrets/:key      # Delete secret
GET    /api/orgs/:slug/projects/:name/envs/:env/secrets/:key/history  # History
POST   /api/orgs/:slug/projects/:name/envs/:env/secrets/:key/restore  # Restore
```

### Export/Import
```
POST   /api/orgs/:slug/projects/:name/envs/:env/import    # Import .env
GET    /api/orgs/:slug/projects/:name/envs/:env/export    # Export secrets
```

### Billing
```
GET    /api/orgs/:slug/billing              # Get billing info
POST   /api/orgs/:slug/billing/portal       # Stripe portal URL
POST   /webhooks/stripe                     # Stripe webhooks
```

## Frontend Pages

```
/                           # Landing page (marketing)
/login                      # Clerk login
/signup                     # Clerk signup

/dashboard                  # Org selector / create org
/[org]                      # Org dashboard, projects list
/[org]/settings             # Org settings, members, billing
/[org]/[project]            # Project view, environments
/[org]/[project]/[env]      # Secrets list for environment
/[org]/[project]/[env]/[key] # Secret detail, history

/invite/:token              # Accept invitation
```

## Key Implementation Details

### Encryption

Server-side encryption with org-specific keys:
1. Each org has a unique encryption key derived from master key + org_id
2. Secrets encrypted with AES-256-GCM (same as CLI)
3. Keys never sent to client - server decrypts on read
4. For CLI sync: use service tokens (encrypted with user's key)

### CLI Integration

Add commands to CLI for SaaS sync:
```bash
# Login to SaaS
coffer cloud login

# Link local project to cloud org
coffer cloud link --org myteam --project myapp

# Push local secrets to cloud
coffer cloud push --env dev

# Pull cloud secrets to local
coffer cloud pull --env dev

# Use cloud secrets directly (no local vault)
coffer cloud run --org myteam --project myapp --env dev -- npm start
```

### Service Tokens

For CI/CD, generate service tokens:
```bash
# In web UI or CLI
coffer cloud token create --org myteam --project myapp --env prod --name "GitHub Actions"
# Returns: cfr_xxxxxxxxxxxxx

# In CI:
COFFER_TOKEN=cfr_xxxxx coffer cloud run --env prod -- ./deploy.sh
```

## MVP Scope (v1)

### Must Have
- [ ] Clerk auth with org management
- [ ] Create/list/delete projects
- [ ] Create/list/delete environments
- [ ] CRUD secrets (encrypted)
- [ ] Basic web UI for viewing/editing secrets
- [ ] Stripe billing (free tier + paid)
- [ ] CLI `cloud login/push/pull` commands

### Nice to Have (v1.1)
- [ ] Environment branching in UI
- [ ] Secret history/versioning in UI
- [ ] Audit log viewer
- [ ] Service tokens for CI/CD
- [ ] Import/export in UI
- [ ] Role-based access control (admin vs member)

### Future (v2)
- [ ] Secret references (${VAR} syntax)
- [ ] GitHub/GitLab integration
- [ ] Slack notifications
- [ ] SSO (SAML/OIDC)
- [ ] Self-hosted enterprise option

## File Structure

```
coffer-saas/
├── frontend/                 # SvelteKit app
│   ├── src/
│   │   ├── routes/
│   │   │   ├── +page.svelte           # Landing
│   │   │   ├── dashboard/
│   │   │   ├── [org]/
│   │   │   │   ├── +page.svelte       # Projects list
│   │   │   │   ├── settings/
│   │   │   │   └── [project]/
│   │   │   │       ├── +page.svelte   # Environments
│   │   │   │       └── [env]/
│   │   │   │           └── +page.svelte  # Secrets
│   │   │   └── invite/[token]/
│   │   ├── lib/
│   │   │   ├── components/
│   │   │   └── api.ts
│   │   └── app.html
│   ├── package.json
│   └── svelte.config.js
│
├── backend/                  # Rust API
│   ├── src/
│   │   ├── main.rs
│   │   ├── routes/
│   │   │   ├── mod.rs
│   │   │   ├── auth.rs
│   │   │   ├── orgs.rs
│   │   │   ├── projects.rs
│   │   │   ├── envs.rs
│   │   │   ├── secrets.rs
│   │   │   └── billing.rs
│   │   ├── models/
│   │   ├── crypto/           # Port from Go or use rust-crypto
│   │   └── db/
│   ├── Cargo.toml
│   └── fly.toml
│
├── cli/                      # Cloud commands (add to existing coffer)
│   └── cmd/
│       └── cloud.go
│
└── README.md
```

## Timeline Estimate

Not providing time estimates - here's the work broken down:

**Phase 1: Foundation**
- Set up SvelteKit frontend with Clerk
- Set up Rust backend with Turso
- Basic org/project/env CRUD
- Secrets CRUD with encryption

**Phase 2: Core Features**
- Full web UI for secrets management
- CLI cloud commands
- Service tokens

**Phase 3: Billing**
- Stripe integration
- Usage tracking
- Upgrade/downgrade flows

**Phase 4: Polish**
- Audit logs
- Better permissions
- Import/export UI
