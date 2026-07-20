# Aurigon Security

A Windows local account management platform. Deploy a lightweight agent on
any Windows machine and manage local user accounts, groups, and sessions
from a central multi-tenant dashboard.

## What it does

- **Account lifecycle** — create, delete, rename, enable/disable, reset
  password, require password change at next logon, unlock, set password
  expiration, set account expiration
- **Group management** — create/delete local groups, add/remove members
- **Session visibility** — see who's actively logged into a machine, force
  logoff
- **Account insight** — full name, password policy state, days since last
  logon, built-in vs. real account detection, group membership, stale
  account and blank-password risk flags
- **Machine health** — OS build, uptime, disk/RAM, pending reboot,
  Windows Defender status, local password policy, failed logon attempts
- **Bulk actions** — select multiple accounts and act on all at once
- **Multi-tenant** — each tenant gets an isolated SQLite database; a single
  permanent agent key (not a rotating credential list) identifies which
  tenant an agent belongs to

## Architecture

```
Agent (Go)  --HTTP-->  Backend (Go)  <-- serves --  Dashboard (Svelte, compiled to static files)
runs on managed          :8080                       same origin, no separate server
Windows machines
```

- **Agent** — Go binary installed as a Windows service (via NSSM). Polls
  the backend on a configurable interval (default 30s), uploads inventory,
  executes queued actions, then immediately re-syncs so results reflect in
  the dashboard within seconds rather than waiting for the next cycle.
- **Backend** — Go HTTP API. One master SQLite database tracks tenants;
  each tenant has its own isolated SQLite database for machines, accounts,
  groups, sessions, and the action queue. Also serves the compiled
  dashboard as static files, so there's no separate frontend server in
  production.
- **Dashboard** — Svelte web UI. JWT-authenticated, tenant-scoped.

## Project layout

```
agent/           Go agent — internal/accounts (enumeration + actions),
                 internal/client (HTTP client), internal/service (main loop)
backend/         Go API server — one file per concern (auth, tenants,
                 users, groups, agent keys, main routing)
dashboard/       Svelte frontend — src/pages, src/components, src/lib
installer/       Inno Setup scripts for the agent and server installers
```

## Prerequisites

- Go 1.21+
- Node.js 18+
- Windows machine(s) for the agent (backend/dashboard can run on Windows too)

## Running locally

### 1. Build the dashboard

```powershell
cd dashboard
npm install
npm run build          # outputs to ../backend/dist
```

### 2. Start the backend

```powershell
cd backend
$env:AURIGON_JWT_SECRET     = "<32+ char secret>"
$env:AURIGON_ADMIN_PASSWORD = "<initial admin password>"
$env:AURIGON_TENANT_SLUG    = "default"
$env:AURIGON_TENANT_NAME    = "My Workspace"
go run .
```

On first run this provisions a default tenant. Backend + dashboard are now
both served at **http://localhost:8080**.

### 3. Get an agent key

Log into the dashboard, go to **Download Agent**, and copy the permanent
agent key shown there. It never changes for that tenant — the same key
works for every machine you deploy the agent to.

### 4. Deploy the agent

```powershell
cd agent
go build -o aurigon-agent.exe ./cmd/agent
$env:AURIGON_AGENT_KEY = "<key from step 3>"
.\aurigon-agent.exe debug     # run in foreground to verify it registers
```

For production, install it as a Windows service via the Inno Setup
installer in `installer/aurigon-agent.iss`, or manually with NSSM.

## Environment variables

| Variable | Component | Purpose |
|---|---|---|
| `AURIGON_JWT_SECRET` | Backend | Signs dashboard session tokens. 32+ chars, required. |
| `AURIGON_ADMIN_PASSWORD` | Backend | Initial admin password when provisioning the first tenant. |
| `AURIGON_TENANT_SLUG` / `AURIGON_TENANT_NAME` | Backend | Identity of the auto-provisioned first tenant. |
| `AURIGON_PORT` | Backend | Defaults to 8080. |
| `AURIGON_AGENT_KEY` | Agent | The permanent per-tenant key from the Download Agent page. Encodes the backend URL and tenant identity. |
| `AURIGON_SYNC_INTERVAL_SECONDS` | Agent | How often the agent polls (default 30, minimum 10). |

## Security notes

- Passwords are bcrypt-hashed; never stored or logged in plaintext.
- New account passwords and password resets are passed to PowerShell via
  environment variables rather than command-line arguments, so they don't
  appear in process listings.
- All SQL is parameterized — no string-built queries.
- Login is rate-limited (5 attempts / 15 min per IP).
- **Not yet done:** the backend currently serves plain HTTP. Before any
  real deployment, put it behind HTTPS (e.g. a reverse proxy with a
  Let's Encrypt certificate) — credentials and account data currently
  travel unencrypted between agent, dashboard, and backend.

## Status

Actively developed, not yet deployed to a public server. See the security
notes above before using this outside a local/trusted network.
