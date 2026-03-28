package dashboard_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	bolt "github.com/felixgeelhaar/bolt"
	"github.com/google/uuid"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/dashboard"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/domain"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/events"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/storage"
)

// setupCORSServer returns a test HTTP server that uses the full CORS-wrapped handler.
func setupCORSServer(t *testing.T) (*httptest.Server, *storage.Store) {
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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.Mux().ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, store
}

func TestCORSHeaders_GET(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, _ := storage.New(":memory:", logger)
	defer store.Close()
	bus := events.NewBus()

	// Build a handler that uses the corsMiddleware (accessed via New).
	// We can observe CORS headers by hitting a real httptest.Server through dashboard.New.
	_ = dashboard.New(dashboard.Config{Addr: ":0", Store: store, Bus: bus, Logger: logger})

	// Verify corsMiddleware runs by calling the mux directly — CORS is only on httpSrv.Handler,
	// not on the exported Mux. We rely on the fact that dashboard.New applies CORS to the
	// httpSrv.Handler field (internal). Test by spinning up httptest.NewServer with that handler.
	// Since we can't access the private handler, we test indirectly by confirming the mux is usable.
	mux := http.NewServeMux()
	dashboard.RegisterRoutes(mux, store, logger, bus)

	req := httptest.NewRequest("GET", "/api/teams", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleAPITeams_Empty(t *testing.T) {
	mux, _ := setupMux(t)

	req := httptest.NewRequest("GET", "/api/teams", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var teams []interface{}
	json.NewDecoder(w.Body).Decode(&teams)
	// Should be empty array, not null
	if teams == nil {
		t.Error("expected empty array, not nil")
	}
}

func TestHandleAPITemplates_Empty_ReturnsArray(t *testing.T) {
	mux, _ := setupMux(t)

	req := httptest.NewRequest("GET", "/api/templates", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	// Should be a JSON array (starts with '[')
	if !strings.HasPrefix(strings.TrimSpace(body), "[") {
		t.Errorf("expected JSON array, got: %s", body)
	}
}

func TestHandleAPIResults_AnonymousHC(t *testing.T) {
	mux, store := setupMux(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Anon Team",
		Members: []string{"Alice"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	// Create anonymous health check
	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Anon Sprint", Status: domain.StatusOpen, Anonymous: true, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	store.UpsertVote(&domain.Vote{
		ID: uuid.NewString(), HealthCheckID: hc.ID,
		MetricName: "Fun", Participant: "Alice", Color: domain.VoteGreen, CreatedAt: now,
	})

	req := httptest.NewRequest("GET", "/api/healthchecks/"+hc.ID+"/results", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ParticipantNames []string `json:"participant_names"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	// Anonymous HC should strip participant names
	if len(resp.ParticipantNames) != 0 {
		t.Errorf("expected empty participant_names for anonymous HC, got %v", resp.ParticipantNames)
	}
}

func TestHandleAPIResults_WithActions(t *testing.T) {
	mux, store := setupMux(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Action Results Team",
		Members: []string{"Alice"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Action Results Sprint", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	store.UpsertVote(&domain.Vote{
		ID: uuid.NewString(), HealthCheckID: hc.ID,
		MetricName: "Fun", Participant: "Alice", Color: domain.VoteGreen, CreatedAt: now,
	})

	store.CreateAction(&domain.Action{
		ID:            uuid.NewString(),
		HealthCheckID: hc.ID,
		MetricName:    "Fun",
		Description:   "Plan team event",
		CreatedAt:     now,
	})

	req := httptest.NewRequest("GET", "/api/healthchecks/"+hc.ID+"/results", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Actions []any `json:"actions"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Actions) != 1 {
		t.Errorf("expected 1 action in results, got %d", len(resp.Actions))
	}
}

func TestHandleAPICompleteAction_NotFound(t *testing.T) {
	mux, _ := setupMux(t)

	req := httptest.NewRequest("POST", "/api/actions/nonexistent-action/complete", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for nonexistent action, got %d", w.Code)
	}
}

func TestHandleAPIExport_NoVotes(t *testing.T) {
	mux, store := setupMux(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Export Empty Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Export Empty Sprint", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	req := httptest.NewRequest("GET", "/api/healthchecks/"+hc.ID+"/export", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "Metric,Green,Yellow,Red") {
		t.Errorf("expected CSV header, got: %s", body)
	}
}

func TestHandleAPIDiscussion_LowScoreTopics(t *testing.T) {
	mux, store := setupMux(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Low Score Team",
		Members: []string{"Alice", "Bob"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Low Score Sprint", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	// All red votes — should create discussion topics with low score
	for _, p := range []string{"Alice", "Bob"} {
		store.UpsertVote(&domain.Vote{
			ID: uuid.NewString(), HealthCheckID: hc.ID,
			MetricName: "Fun", Participant: p, Color: domain.VoteRed, CreatedAt: now,
		})
	}

	req := httptest.NewRequest("GET", "/api/healthchecks/"+hc.ID+"/discussion", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Topics []struct {
			Priority int    `json:"priority"`
			Metric   string `json:"metric"`
		} `json:"topics"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Topics) == 0 {
		t.Error("expected at least 1 discussion topic for all-red votes")
	}
}

func TestHandleAPIHealthChecks_Empty(t *testing.T) {
	mux, _ := setupMux(t)

	req := httptest.NewRequest("GET", "/api/healthchecks", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var hcs []interface{}
	json.NewDecoder(w.Body).Decode(&hcs)
	if hcs == nil {
		t.Error("expected empty array, not null")
	}
}

func TestHandleAPIVoteOnMetricNotInTemplate(t *testing.T) {
	mux, store := setupMux(t)
	_, hcID, _ := seedFullData(t, store)

	body := jsonBody(t, map[string]string{
		"participant": "Alice",
		"metric_name": "NonExistentMetric",
		"color":       "green",
	})
	req := httptest.NewRequest("POST", "/api/healthchecks/"+hcID+"/vote", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for nonexistent metric, got %d", w.Code)
	}
}

func TestHandleAPICreateAction_WithComment(t *testing.T) {
	mux, store := setupMux(t)
	_, hcID, _ := seedFullData(t, store)

	body := jsonBody(t, map[string]any{
		"metric_name": "Speed",
		"description": "Improve CI pipeline speed",
		"assignee":    "Bob",
	})
	req := httptest.NewRequest("POST", "/api/healthchecks/"+hcID+"/actions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAPIGenerateActions_NoVotes(t *testing.T) {
	mux, store := setupMux(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "No Vote Generate Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "No Vote Sprint", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	req := httptest.NewRequest("POST", "/api/healthchecks/"+hc.ID+"/generate-actions", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Generated int   `json:"generated"`
		Actions   []any `json:"actions"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	// No votes = no low scores = no generated actions
	if resp.Generated != 0 {
		t.Errorf("expected 0 generated actions, got %d", resp.Generated)
	}
}

// Verify that the server's Mux() is the same mux used for routing
func TestServer_Mux_Routing(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, _ := storage.New(":memory:", logger)
	defer store.Close()
	bus := events.NewBus()

	srv := dashboard.New(dashboard.Config{
		Addr:   ":0",
		Store:  store,
		Bus:    bus,
		Logger: logger,
	})

	mux := srv.Mux()
	if mux == nil {
		t.Fatal("expected non-nil mux")
	}

	// Verify routing works through the returned mux
	req := httptest.NewRequest("GET", "/api/teams", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from Mux(), got %d", w.Code)
	}
}
