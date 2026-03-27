# Changelog

## [v1.0.0] (unreleased)

### Breaking Changes

- Complete pivot from Go library to MCP server product
- Removed `Indicator` and `HealthMetric` exported types (replaced by internal domain model)
- Module now provides `cmd/healthcheck-mcp` binary instead of importable package

### Features

- MCP server with 24 tools for full health check lifecycle
- Dual transport: stdio (single-user) and HTTP/SSE (multi-user)
- Token-based authentication with auto-filled participant identity
- `my_pending_healthchecks` tool for authenticated users
- statekit-powered state machine: open → closed → archived lifecycle
- Guards enforce business rules (can't close without votes)
- `reopen_healthcheck` and `archive_healthcheck` tools
- bolt structured logging (JSON prod, colored console dev)
- fortify resilience middleware via mcp-go (rate limiting, timeouts)
- SQLite persistence (pure Go, no CGO) with WAL mode and busy timeout
- Built-in Spotify Squad Health Check template (10 metrics)
- Custom template creation
- Team management with member tracking
- Vote submission with upsert semantics (one vote per participant per metric)
- Aggregated results with score computation (1-3 scale)
- Cross-session comparison with trend detection (improving/stable/declining)
- AI-friendly analysis: strengths, concerns, discussion topics
- Discussion topic generation based on disagreement, low scores, and declining trends
- Live web dashboard with real-time updates via WebSocket (React SPA, `--dashboard-addr :3000`)
- Event bus architecture: Store publishes events on mutations, dashboard auto-updates
- MCP Apps UIResource: interactive voting form and results heatmap (rendered in Claude Desktop)
- Dashboard REST API: `/api/teams`, `/api/healthchecks`, `/api/healthchecks/{id}/results`
- SPA embedded in binary via `embed.FS` — single binary deployment
- Dark glassmorphism UI design (Linear/Vercel aesthetic)
- Web-based voting with metric descriptions (good/bad anchors visible)
- Spotify metric picker: click individual metrics or "Add All" for custom templates
- Radar/spider chart: SVG at-a-glance visualization of all metric scores
- AI discussion guide: surfaces disagreement patterns and low scores with suggested questions
- Anonymous voting: toggle per health check, strips names and comments from results
- Team health trends: bar chart + per-metric sparklines with tendency indicators
- Comments visible in expanded metric rows
- Participant avatars with colored initials
- Pre-commit hooks: go fmt, go vet, go test, nox security scan, coverctl coverage

## [v0.1.1](https://github.com/FelixGeelhaar/go-teamhealthcheck/releases/tag/v0.1.1) (2020-04-14)

Github Action, Documentation and several documentation updates

## [v0.1.0](https://github.com/FelixGeelhaar/go-teamhealthcheck/releases/tag/v0.1.0) (2020-04-14)

### Features

- Added the inital model for the Health metric and the traffic light indicators
  ([56ebeee](https://github.com/FelixGeelhaar/go-teamhealthcheck/commit/56ebeee7a7e8a6e4d92aa00136698e8a726d6a0a))

### Bugs

No bugs smashed
