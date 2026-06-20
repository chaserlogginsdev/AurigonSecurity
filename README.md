# AurigonSecurity Agent

AurigonSecurity is a lightweight Windows endpoint agent designed for local account enumeration, security posture reporting, and remote action execution. This repository contains the Go-based agent that runs as a Windows service and communicates with the Aurigon backend.

## Features

- Enumerates local Windows accounts (users, SIDs, admin status, last logon)
- Sends inventory to backend API
- Polls for remote actions
- Executes actions (disable account, remove from admin group, etc.)
- Designed for MSI deployment
- Multi-tenant support via API key or config file

## Project Structure

AurigonSecurity/
├── cmd/
│   └── agent/          # Main entrypoint for the agent
├── internal/
│   ├── accounts/       # Account enumeration logic
│   ├── actions/        # Action execution handlers
│   ├── client/         # HTTP client for backend communication
│   ├── config/         # Config loader (API key, backend URL)
│   └── service/        # Main agent loop
├── pkg/
│   └── util/           # Shared utilities
└── go.mod


## Getting Started

### Build the agent

go build -o aurigon-agent.exe ./cmd/agent

### Run locally

.\aurigon-agent.exe

### Mock Backend

A mock backend is available for local development and testing.

## Roadmap

- Device registration flow
- Inventory upload
- Action polling
- Windows service integration
- MSI installer
- Tenant API key support
- Production backend

## License

MIT (or your preferred license)

# Check repo status
git status

# Add all changes
git add .

# Commit changes with a message
git commit -m "your message here"

# Push to GitHub
git push

# Pull latest changes (if working on multiple machines)
git pull

# View commit history
git log --oneline --graph --decorate

#Agent Commands
cd AurigonAgent
go build -o aurigon-agent.exe ./cmd/agent
.\aurigon-agent.exe

# Backend Commands
cd AurigonBackend
go run main.go

