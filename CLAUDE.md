# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MCP server for running Spotify Squad Health Checks with AI agents. Supports single-user (stdio) and multi-user (HTTP/SSE) modes with token-based auth, a live web dashboard with real-time updates, and MCP Apps UI for in-client voting.

Built with:
- `github.com/felixgeelhaar/mcp-go` — MCP framework
- `github.com/felixgeelhaar/bolt` — Structured logging
- `github.com/felixgeelhaar/statekit` — State machine for health check lifecycle
- `modernc.org/sqlite` — Pure Go SQLite
- React + Vite — Dashboard SPA (embedded in binary)

## Commands

```bash
# Build the binary
go build ./cmd/healthcheck-mcp/

# Run all tests
go test -race ./...

# Run a single test
go test -race -run TestTransition_OpenToClosedWithVotes ./internal/domain/

# Vet
go vet ./...

# Run stdio mode
go run ./cmd/healthcheck-mcp/ --db /tmp/test.db

# Run HTTP mode with dashboard
go run ./cmd/healthcheck-mcp/ --mode http --addr :8080 --dashboard-addr :3000 --dev

# Build the React SPA (required before go build if SPA changed)
cd web/app && npm install && ./node_modules/.bin/tsc && ./node_modules/.bin/vite build
cp dist/index.html ../../internal/dashboard/spa/index.html

# Run pre-commit hooks manually
.githooks/pre-commit
```

## Architecture

```
cmd/healthcheck-mcp/main.go       → Entry point: bolt logger, dual transport, event bus, dashboard server
internal/auth/config.go            → Auth config loader (token → user identity mapping)
internal/domain/                   → Pure domain: entities, value objects, state machine
internal/events/bus.go             → Event bus: pub/sub for real-time updates
internal/storage/                  → SQLite repos (publishes events on mutations)
internal/mcp/                      → 24 MCP tools + UIResource registrations
internal/dashboard/                → Dashboard HTTP server: REST API + WebSocket hub + embedded SPA
internal/mcpui/                    → Self-contained HTML generators for MCP Apps
internal/seed/                     → Built-in Spotify template data
web/app/                           → React SPA source (Vite + TypeScript)
```

**Real-time flow**: MCP tool call → Store mutation → EventBus.Publish → WebSocket Hub.OnEvent → broadcast to all browser clients → React refetches via REST API.

**Domain layer** (`internal/domain/`) has zero infrastructure imports except bolt and statekit.

**Key domain types:**
- `Team` — aggregate root, owns health checks
- `Template` / `TemplateMetric` — reusable health check formats
- `HealthCheck` — session aggregate with open/closed/archived lifecycle via statekit
- `Vote` — participant's green/yellow/red choice (upsert on re-vote)
- `MetricResult` / `MetricTrend` — computed views, not stored
- `HealthCheckStateMachine` — statekit-powered lifecycle transitions with guards and actions

**State machine:** Two machine configs (open→closed with hasVotes guard, closed→archived/reopened). The `Transition()` method selects the right machine based on current status.

**Dashboard:** Separate HTTP server on `:3000`. React SPA (dark glassmorphism) built to single HTML file via vite-plugin-singlefile, embedded in Go binary via `embed.FS`. Features: radar chart, discussion guide, anonymous voting, web voting with metric descriptions, Spotify metric picker, team health trends, comments display. REST API + WebSocket for live updates.

**HealthCheck.Anonymous:** When true, results API strips participant names and comments. Set at creation time via dashboard or MCP tool.

**Storage:** SQLite via `modernc.org/sqlite`. WAL mode + busy_timeout for concurrent access. Event bus publishes after mutations.

## Pre-commit Hooks

Git hooks in `.githooks/` (configured via `git config core.hooksPath .githooks`):
1. `go fmt` — formatting
2. `go vet` — static analysis
3. `go build` — compilation
4. `go test -race` — tests with race detector
5. `nox scan` — security scan (baseline at `.nox/baseline.json`)
6. `coverctl check` — coverage thresholds (`.coverctl.yaml`)

## Commit Convention

Conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `perf:`, `test:`. Subject max 50 chars, imperative mood.
