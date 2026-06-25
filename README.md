# Aurigon

A lightweight IT account management platform. Deploy the agent on any Windows machine and manage local user accounts from a central dashboard.

## What it does

- Enumerates local Windows accounts (username, SID, admin status, enabled/disabled, last logon)
- Displays all accounts per machine in a web dashboard
- Lets admins disable or enable accounts remotely
- Persists data across restarts with SQLite

## Architecture

```
AurigonDashboard (Svelte)  ←→  AurigonBackend (Go)  ←→  aurigon-agent (Go)
     localhost:5173               localhost:8080          runs on managed machines
```

- **Dashboard** — Svelte web UI. Login protected with JWT.
- **Backend** — Go HTTP API. Stores machines, accounts, and action queue in SQLite.
- **Agent** — Go binary deployed on each managed machine. Polls backend every 30s, uploads account inventory, executes queued actions.

## Prerequisites

- Go 1.21+
- Node.js 18+

## Running locally

### 1. Start the backend

```powershell
cd AurigonBackend
go build -o aurigon-backend.exe .
.\aurigon-backend.exe
```

On first run it creates `aurigon.db` and a default admin account:
- Username: `admin`
- Password: `admin123`

### 2. Start the agent (requires Administrator)

Open an elevated PowerShell window:

```powershell
go build -o aurigon-agent.exe ./cmd/agent
.\aurigon-agent.exe
```

The agent registers with the backend, enumerates local accounts, and uploads them every 30 seconds.

### 3. Start the dashboard

```powershell
cd AurigonDashboard
npm install
npm run dev
```

Open `http://localhost:5173` and sign in with `admin` / `admin123`.

## Project structure

```
AurigonSecurity/
├── cmd/agent/main.go          # Agent entry point
├── internal/
│   ├── accounts/              # Windows account enumeration (PowerShell)
│   ├── client/                # Agent HTTP client
│   └── service/               # Agent main loop (register, enumerate, poll)
├── AurigonBackend/
│   ├── main.go                # HTTP API routes and handlers
│   ├── db.go                  # SQLite schema and init
│   └── auth.go                # JWT login and middleware
└── AurigonDashboard/
    └── src/App.svelte         # Dashboard UI
```

## Environment variables (backend)

| Variable | Default | Description |
|---|---|---|
| `AURIGON_JWT_SECRET` | `aurigon-dev-secret-...` | JWT signing secret — change in production |
| `AURIGON_ADMIN_PASSWORD` | `admin123` | Password for the default admin user (only used on first run) |

## Roadmap

- [ ] Change password UI
- [ ] Linux and macOS agent support
- [ ] Add / delete accounts
- [ ] Local group management
- [ ] Production deployment guide
