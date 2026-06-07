package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.klarlabs.de/bolt"
	_ "modernc.org/sqlite"

	"github.com/felixgeelhaar/heartbeat/internal/events"
	"github.com/felixgeelhaar/heartbeat/internal/seed"
	"github.com/google/uuid"
)

// Store holds the database connection and implements all repository interfaces.
type Store struct {
	db     *sql.DB
	logger *bolt.Logger
	bus    *events.Bus
}

// SetEventBus attaches an event bus to the store for publishing data change events.
func (s *Store) SetEventBus(bus *events.Bus) {
	s.bus = bus
}

// publish is a nil-safe helper for emitting events.
func (s *Store) publish(e events.Event) {
	if s.bus != nil {
		s.bus.Publish(e)
	}
}

// New opens (or creates) a SQLite database at the given path, runs migrations,
// and seeds built-in data. Pass ":memory:" for an in-memory database.
func New(dbPath string, logger *bolt.Logger) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	// For in-memory databases, use shared cache so all connections in the pool
	// see the same database. Without this, each pooled connection gets a
	// separate empty in-memory database.
	dsn := dbPath
	if dbPath == ":memory:" {
		dsn = "file::memory:?cache=shared"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode, foreign keys, and busy timeout for concurrent access
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("set pragma: %w", err)
		}
	}

	s := &Store{db: db, logger: logger}

	logger.Debug().Str("path", dbPath).Msg("running migrations")
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	if err := s.seedDefaults(); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed defaults: %w", err)
	}
	logger.Info().Str("path", dbPath).Msg("storage initialized")

	return s, nil
}

// DB returns the underlying database handle for plugin access.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) seedDefaults() error {
	for _, tmpl := range seed.AllBuiltInTemplates() {
		existing, err := s.FindTemplateByName(tmpl.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}

		tmpl.ID = uuid.NewString()
		for i := range tmpl.Metrics {
			tmpl.Metrics[i].ID = uuid.NewString()
			tmpl.Metrics[i].TemplateID = tmpl.ID
		}

		s.logger.Debug().Str("template", tmpl.Name).Msg("seeding built-in template")
		if err := s.CreateTemplate(&tmpl); err != nil {
			return err
		}
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS teams (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS team_members (
    id      TEXT PRIMARY KEY,
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name    TEXT NOT NULL,
    UNIQUE(team_id, name)
);

CREATE TABLE IF NOT EXISTS templates (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    built_in    BOOLEAN NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS template_metrics (
    id               TEXT PRIMARY KEY,
    template_id      TEXT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    description_good TEXT NOT NULL DEFAULT '',
    description_bad  TEXT NOT NULL DEFAULT '',
    sort_order       INTEGER NOT NULL DEFAULT 0,
    UNIQUE(template_id, name)
);

CREATE TABLE IF NOT EXISTS healthchecks (
    id          TEXT PRIMARY KEY,
    team_id     TEXT NOT NULL REFERENCES teams(id),
    template_id TEXT NOT NULL REFERENCES templates(id),
    name        TEXT NOT NULL,
    anonymous   BOOLEAN NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open', 'closed', 'archived')),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at   DATETIME
);

CREATE INDEX IF NOT EXISTS idx_healthchecks_team ON healthchecks(team_id);
CREATE INDEX IF NOT EXISTS idx_healthchecks_team_created ON healthchecks(team_id, created_at);

CREATE TABLE IF NOT EXISTS votes (
    id              TEXT PRIMARY KEY,
    healthcheck_id  TEXT NOT NULL REFERENCES healthchecks(id) ON DELETE CASCADE,
    metric_name     TEXT NOT NULL,
    participant     TEXT NOT NULL,
    color           TEXT NOT NULL CHECK(color IN ('green', 'yellow', 'red')),
    comment         TEXT NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(healthcheck_id, metric_name, participant)
);

CREATE INDEX IF NOT EXISTS idx_votes_healthcheck ON votes(healthcheck_id);

CREATE TABLE IF NOT EXISTS actions (
    id              TEXT PRIMARY KEY,
    healthcheck_id  TEXT NOT NULL REFERENCES healthchecks(id) ON DELETE CASCADE,
    metric_name     TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL,
    assignee        TEXT NOT NULL DEFAULT '',
    completed       BOOLEAN NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_actions_healthcheck ON actions(healthcheck_id);
`
