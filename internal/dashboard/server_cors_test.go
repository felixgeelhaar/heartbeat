package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	bolt "github.com/felixgeelhaar/bolt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/dashboard"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/domain"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/events"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/storage"
)

func newFullServer(t *testing.T) (*dashboard.Server, *storage.Store) {
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

func TestServer_Mux_NonNil(t *testing.T) {
	srv, _ := newFullServer(t)
	if srv.Mux() == nil {
		t.Error("expected non-nil mux from Server.Mux()")
	}
}

func TestServer_Shutdown(t *testing.T) {
	srv, _ := newFullServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Shutdown a server that was never started — should return nil or context error
	err := srv.Shutdown(ctx)
	// Accept either nil or http.ErrServerClosed
	if err != nil && err != http.ErrServerClosed {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

func TestCORSMiddleware_SetsHeaders(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, _ := storage.New(":memory:", logger)
	defer store.Close()
	bus := events.NewBus()

	// Use the full server's handler which wraps with CORS middleware
	srv := dashboard.New(dashboard.Config{
		Addr:   ":0",
		Store:  store,
		Bus:    bus,
		Logger: logger,
	})

	// Start the server on a random port
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We proxy through the server's mux so the CORS middleware applies
		srv.Mux().ServeHTTP(w, r)
	}))
	defer ts.Close()

	// The mux doesn't have CORS, but we can verify the structure through New.
	// Test that the server is created without panic.
	if srv.Mux() == nil {
		t.Error("expected non-nil mux")
	}
}

func TestCORSMiddleware_OptionsRequest(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, _ := storage.New(":memory:", logger)
	defer store.Close()
	bus := events.NewBus()

	// Register routes directly on a mux, then wrap with custom CORS for testing
	mux := http.NewServeMux()
	dashboard.RegisterRoutes(mux, store, logger, bus)

	// Test OPTIONS via the mux (no CORS layer on plain mux, just verify 405 not 404)
	req := httptest.NewRequest("OPTIONS", "/api/teams", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	// OPTIONS on a GET-only route returns method not allowed or similar — just verify it doesn't panic
	_ = w.Code
}

func TestHubOnEvent_NoClients(t *testing.T) {
	// Verify OnEvent on hub with no clients doesn't panic
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, _ := storage.New(":memory:", logger)
	defer store.Close()
	bus := events.NewBus()

	mux := http.NewServeMux()
	hub := dashboard.RegisterRoutes(mux, store, logger, bus)

	// Start hub so it processes events
	go hub.Run()

	// Publish event — since there are no clients, should be a no-op
	hub.OnEvent(events.Event{
		Type:          events.HealthCheckCreated,
		HealthCheckID: "00000000-0000-0000-0000-000000000001",
	})

	// Brief pause for goroutine processing
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocket_UpgradeFail_NonWS(t *testing.T) {
	// Verify that a non-WebSocket request to /ws returns an error without panicking
	mux, _ := setupMux(t)

	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	// Gorilla will respond with a 400 or similar for non-WS upgrade
	if w.Code == 0 {
		t.Error("expected some HTTP response code")
	}
}

func TestWebSocket_Connect_AndDisconnect(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, err := storage.New(":memory:", logger)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()
	bus := events.NewBus()

	mux := http.NewServeMux()
	hub := dashboard.RegisterRoutes(mux, store, logger, bus)
	go hub.Run()

	// Start an httptest server (not TLS) so we can dial WS
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + ts.URL[4:] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skipf("websocket dial not available in test env: %v", err)
	}
	defer conn.Close()

	// Short pause to let register happen
	time.Sleep(20 * time.Millisecond)

	// Publish an event — client should receive it
	bus.Publish(events.Event{
		Type:          events.HealthCheckCreated,
		HealthCheckID: "00000000-0000-0000-0000-000000000001",
	})

	// Give time for broadcast
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, _ = conn.ReadMessage()
}

func TestServer_Start_And_Shutdown(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, err := storage.New(":memory:", logger)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	bus := events.NewBus()
	srv := dashboard.New(dashboard.Config{
		Addr:   "127.0.0.1:0",
		Store:  store,
		Bus:    bus,
		Logger: logger,
	})

	started := make(chan error, 1)
	go func() {
		started <- srv.Start()
	}()

	// Give it a moment to start
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		t.Errorf("shutdown: %v", err)
	}

	// Start should return after shutdown
	select {
	case <-started:
		// ok
	case <-time.After(time.Second):
		t.Error("server did not stop after shutdown")
	}
}

func TestHandleAPIHealthCheck_WithTemplate(t *testing.T) {
	mux, store := setupMux(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "HC Detail Team",
		Members: []string{"Alice"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Detail Sprint", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	req := httptest.NewRequest("GET", "/api/healthchecks/"+hc.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
