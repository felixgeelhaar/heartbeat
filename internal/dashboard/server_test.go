package dashboard_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	bolt "go.klarlabs.de/bolt"

	"github.com/felixgeelhaar/heartbeat/internal/dashboard"
	"github.com/felixgeelhaar/heartbeat/internal/domain"
	"github.com/felixgeelhaar/heartbeat/internal/events"
	"github.com/felixgeelhaar/heartbeat/internal/storage"
)

func setupDashboard(t *testing.T) (*dashboard.Server, *storage.Store) {
	t.Helper()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, err := storage.New(":memory:", logger)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	bus := events.NewBus()
	store.SetEventBus(bus)

	srv := dashboard.New(dashboard.Config{
		Addr:   ":0",
		Store:  store,
		Bus:    bus,
		Logger: logger,
	})
	return srv, store
}

func seedTestData(t *testing.T, store *storage.Store) (teamID, hcID string) {
	t.Helper()
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")

	team := &domain.Team{
		ID: uuid.NewString(), Name: "Dashboard Team",
		Members: []string{"Alice", "Bob"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Sprint 1", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	store.UpsertVote(&domain.Vote{
		ID: uuid.NewString(), HealthCheckID: hc.ID,
		MetricName: "Fun", Participant: "Alice", Color: domain.VoteGreen, CreatedAt: now,
	})

	return team.ID, hc.ID
}

func TestAPITeams(t *testing.T) {
	srv, store := setupDashboard(t)
	_ = srv
	seedTestData(t, store)

	// Use the handler directly via httptest
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	bus := events.NewBus()
	dashSrv := dashboard.New(dashboard.Config{
		Addr: ":0", Store: store, Bus: bus, Logger: logger,
	})
	_ = dashSrv

	req := httptest.NewRequest("GET", "/api/teams", nil)
	w := httptest.NewRecorder()

	// We need to test the handler directly. Create a fresh server and use its internal mux.
	// Since the Server wraps an http.Server, let's test via the exported New + Start pattern.
	// For unit tests, test the handler functions directly:
	mux := http.NewServeMux()
	dashboard.RegisterRoutes(mux, store, bolt.New(bolt.NewConsoleHandler(os.Stderr)), bus)

	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var teams []struct{ ID, Name string }
	json.NewDecoder(w.Body).Decode(&teams)
	if len(teams) != 1 {
		t.Errorf("expected 1 team, got %d", len(teams))
	}
}

func TestAPIResults(t *testing.T) {
	srv, store := setupDashboard(t)
	_ = srv
	_, hcID := seedTestData(t, store)

	mux := http.NewServeMux()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	bus := events.NewBus()
	dashboard.RegisterRoutes(mux, store, logger, bus)

	req := httptest.NewRequest("GET", "/api/healthchecks/"+hcID+"/results", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var results struct {
		TotalVotes int `json:"total_votes"`
	}
	json.NewDecoder(w.Body).Decode(&results)
	if results.TotalVotes != 1 {
		t.Errorf("expected 1 vote, got %d", results.TotalVotes)
	}
}

func TestAPIHealthCheckNotFound(t *testing.T) {
	srv, store := setupDashboard(t)
	_ = srv

	mux := http.NewServeMux()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	bus := events.NewBus()
	dashboard.RegisterRoutes(mux, store, logger, bus)

	req := httptest.NewRequest("GET", "/api/healthchecks/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAPIResultsNotFound(t *testing.T) {
	srv, store := setupDashboard(t)
	_ = srv

	mux := http.NewServeMux()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	bus := events.NewBus()
	dashboard.RegisterRoutes(mux, store, logger, bus)

	req := httptest.NewRequest("GET", "/api/healthchecks/nonexistent/results", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
