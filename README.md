# healthcheck-mcp

An MCP (Model Context Protocol) server for running Spotify Squad Health Checks with AI agents.

Let AI facilitate team health checks directly in conversations — collecting votes, aggregating results, tracking trends, and surfacing discussion topics. Supports both single-user (stdio) and multi-user (HTTP/SSE) modes.

## Features

- **24 MCP tools** — Full health check lifecycle: teams, templates, sessions, voting, comparison, and analysis
- **Live web dashboard** — Dark glassmorphism React SPA with real-time WebSocket updates
- **Radar chart** — SVG spider chart for at-a-glance team health visualization
- **AI discussion guide** — Surfaces disagreement patterns and low scores with suggested retro questions
- **Web voting** — Vote through the dashboard UI with metric descriptions (good/bad anchors)
- **Anonymous voting** — Toggle per health check to strip participant names from results
- **Spotify metric picker** — Click individual Spotify metrics or "Add All" when building custom templates
- **Multi-user HTTP/SSE** — Multiple team members connect to a shared server, each voting independently
- **Team trends** — Track health improvement over time with bar charts and per-metric sparklines
- **Token-based auth** — Bearer token authentication with auto-filled participant identity
- **State machine lifecycle** — statekit-powered transitions: open → closed → archived, with guards
- **MCP Apps** — Interactive voting form and results heatmap rendered in Claude Desktop
- **Structured logging** — bolt-powered JSON (prod) or colored console (dev) logging
- **SQLite storage** — Zero-config persistence, single-file database
- **Single binary** — 24MB binary with embedded SPA, no runtime dependencies

## Installation

### From source

```bash
go install github.com/felixgeelhaar/go-teamhealthcheck/cmd/healthcheck-mcp@latest
```

### From release

Download a pre-built binary from [Releases](https://github.com/felixgeelhaar/go-teamhealthcheck/releases).

## Usage

### Single-user (stdio)

```bash
healthcheck-mcp
```

For use with Claude Desktop, add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "healthcheck": {
      "command": "healthcheck-mcp",
      "args": []
    }
  }
}
```

### Multi-user (HTTP/SSE) with Dashboard

```bash
healthcheck-mcp --mode http --addr :8080 --dashboard-addr :3000
```

Multiple team members connect their MCP clients to the same server. Each authenticates with a Bearer token that maps to their identity. The live dashboard is available at `http://localhost:3000`.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--mode` | `stdio` | Transport: `stdio` or `http` |
| `--addr` | `:8080` | HTTP listen address |
| `--dashboard-addr` | `:3000` | Dashboard HTTP listen address (empty to disable) |
| `--db` | `~/.healthcheck-mcp/data.db` | SQLite database path |
| `--auth` | `~/.healthcheck-mcp/auth.json` | Auth config file (HTTP mode) |
| `--dev` | `false` | Development mode (colored console logging) |

### Auth Configuration

Create `~/.healthcheck-mcp/auth.json`:

```json
{
  "tokens": {
    "alice-secret-token": {"name": "Alice", "team_id": "your-team-uuid"},
    "bob-secret-token": {"name": "Bob", "team_id": "your-team-uuid"}
  }
}
```

When authenticated, `submit_vote` auto-fills the participant name from the token identity. The `my_pending_healthchecks` tool shows which metrics the user hasn't voted on yet.

## MCP Tools

### Team Management
| Tool | Description |
|------|-------------|
| `create_team` | Create a new team with optional initial members |
| `list_teams` | List all teams |
| `get_team` | Get team details including members |
| `delete_team` | Delete a team |
| `add_team_member` | Add a member to a team |
| `remove_team_member` | Remove a member from a team |

### Templates
| Tool | Description |
|------|-------------|
| `list_templates` | List all templates (includes built-in Spotify template) |
| `get_template` | Get template with all metric definitions |
| `create_template` | Create a custom template with metrics |
| `delete_template` | Delete a custom template |

### Health Check Sessions
| Tool | Description |
|------|-------------|
| `create_healthcheck` | Start a new health check session |
| `list_healthchecks` | List sessions (filter by team/status) |
| `get_healthcheck` | Get session with current results |
| `close_healthcheck` | Close session (requires at least one vote) |
| `reopen_healthcheck` | Reopen a closed session for more votes |
| `archive_healthcheck` | Archive a closed session (terminal) |
| `delete_healthcheck` | Delete session and all votes |
| `my_pending_healthchecks` | List metrics the authenticated user hasn't voted on |

### Voting
| Tool | Description |
|------|-------------|
| `submit_vote` | Vote green/yellow/red on a metric (participant auto-filled from auth) |
| `get_results` | Get aggregated results with scores and stats |

### Analysis
| Tool | Description |
|------|-------------|
| `compare_sessions` | Compare results across sprints with trend detection |
| `analyze_healthcheck` | AI-friendly summary with strengths/concerns |
| `get_trends` | Historical trend analysis for a team |
| `get_discussion_topics` | Suggested topics based on disagreement, low scores, and declining trends |

## Health Check Lifecycle

The session lifecycle is managed by a [statekit](https://github.com/felixgeelhaar/statekit) state machine:

```
         create              close (requires votes)        archive
         ──────→  open  ─────────────────────────→  closed  ──────→  archived
                   ↑                                  │
                   └──────────── reopen ──────────────┘
```

- **open** — accepting votes
- **closed** — voting complete, results available, can reopen or archive
- **archived** — terminal state, read-only

Guards enforce business rules (e.g., can't close without votes). Actions execute side effects (set timestamps, log transitions).

## Live Dashboard

The dashboard provides a real-time web UI for viewing health check results. It runs on a separate port alongside the MCP server.

**Features:**
- Team selector with health check list
- Per-metric traffic-light results grid with scores
- Voting progress tracking
- Live updates via WebSocket — when anyone votes, all dashboards update instantly

**How it works:**
1. Store publishes events after mutations (vote submitted, status changed, etc.)
2. WebSocket hub broadcasts events to all connected browser clients
3. React SPA refetches data and re-renders the affected components

The SPA is embedded in the Go binary — no separate frontend deployment needed.

### Dashboard REST API

| Endpoint | Description |
|----------|-------------|
| `GET /api/teams` | List all teams |
| `GET /api/healthchecks?team_id=X&status=Y` | List health checks |
| `GET /api/healthchecks/{id}` | Health check details + template |
| `GET /api/healthchecks/{id}/results` | Aggregated results |
| `GET /ws` | WebSocket for real-time events |

## MCP Apps

When used with Claude Desktop or other MCP Apps-compatible clients, tools can render interactive HTML UIs:

- **Voting form** (`ui://healthcheck/{id}/vote`) — card per metric with green/yellow/red buttons
- **Results heatmap** (`ui://healthcheck/{id}/results`) — traffic-light visualization with scores

These are self-contained HTML documents rendered in a sandboxed iframe.

## Multi-User Workflow

```
# Team lead starts the server with dashboard
healthcheck-mcp --mode http --addr :8080 --dashboard-addr :3000

# Each team member connects their AI client with their token
# Alice's session:
Alice: "Do I have any pending health checks?"
Agent: [calls my_pending_healthchecks] You have Sprint 42 — 10 metrics pending.
Alice: "I vote green on Fun, red on Tech Quality..."
Agent: [calls submit_vote x10] All votes recorded!

# Bob's session (same server):
Bob: "What health checks are open?"
Agent: [calls my_pending_healthchecks] Sprint 42 — 10 metrics pending.
Bob: "Green on everything except Speed, that's yellow"
Agent: [calls submit_vote x10] Done!

# Team lead reviews:
Lead: "Show me the Sprint 42 results"
Agent: [calls get_results] Here's the breakdown...
Lead: "What should we discuss?"
Agent: [calls get_discussion_topics] Top topics: Tech Quality (disagreement)...

# Meanwhile, the dashboard at localhost:3000 shows results updating live
```

## Built-in Spotify Template

Ships with the original [Spotify Squad Health Check](https://labs.spotify.com/2014/09/16/squad-health-check-model/) categories:

1. Easy to Release
2. Suitable Process
3. Tech Quality
4. Value
5. Speed
6. Mission
7. Fun
8. Learning
9. Support
10. Pawns or Players

## Building from Source

```bash
# 1. Build the React SPA (requires Node.js)
cd web/app
npm install
./node_modules/.bin/tsc && ./node_modules/.bin/vite build
cp dist/index.html ../../internal/dashboard/spa/index.html
cd ../..

# 2. Build the Go binary (SPA is embedded)
go build -o healthcheck-mcp ./cmd/healthcheck-mcp/
```

The SPA only needs rebuilding when `web/app/src/` changes. The Go binary embeds the built HTML file, so the final artifact is a single binary with no runtime dependencies.

## Built With

- [mcp-go](https://github.com/felixgeelhaar/mcp-go) — MCP server framework
- [bolt](https://github.com/felixgeelhaar/bolt) — Structured logging
- [statekit](https://github.com/felixgeelhaar/statekit) — State machine engine
- [fortify](https://github.com/felixgeelhaar/fortify) — Resilience middleware (via mcp-go)

## Commit Convention

Conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `perf:`, `test:`.

## License

MIT
