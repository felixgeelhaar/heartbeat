package dashboard

import (
	"context"
	"embed"
	"net/http"

	bolt "go.klarlabs.de/bolt"

	"github.com/felixgeelhaar/heartbeat/internal/events"
	"github.com/felixgeelhaar/heartbeat/internal/storage"
)

//go:embed spa/index.html
var spaFS embed.FS

// Config configures the dashboard server.
type Config struct {
	Addr   string
	Store  *storage.Store
	Bus    *events.Bus
	Logger *bolt.Logger
}

// Server is the dashboard HTTP server with REST API and WebSocket support.
type Server struct {
	mux     *http.ServeMux
	httpSrv *http.Server
	hub     *Hub
	logger  *bolt.Logger
}

// RegisterRoutes adds dashboard API, WebSocket, and SPA routes to a mux.
// Exported for testing with httptest.
func RegisterRoutes(mux *http.ServeMux, store *storage.Store, logger *bolt.Logger, bus *events.Bus) *Hub {
	hub := NewHub(logger)
	bus.Subscribe(hub)

	// REST API — GET
	mux.HandleFunc("GET /api/teams", handleAPITeams(store))
	mux.HandleFunc("GET /api/templates", handleAPITemplates(store))
	mux.HandleFunc("GET /api/healthchecks", handleAPIHealthChecks(store))
	mux.HandleFunc("GET /api/healthchecks/{id}", handleAPIHealthCheck(store))
	mux.HandleFunc("GET /api/healthchecks/{id}/results", handleAPIResults(store))
	mux.HandleFunc("GET /api/teams/{id}/trends", handleAPITeamTrends(store))
	mux.HandleFunc("GET /api/healthchecks/{id}/discussion", handleAPIDiscussion(store))
	mux.HandleFunc("GET /api/healthchecks/{id}/export", handleAPIExport(store))
	mux.HandleFunc("GET /api/compare", handleAPICompare(store))
	mux.HandleFunc("GET /api/teams/{id}/alerts", handleAPIAlerts(store))

	// REST API — POST
	mux.HandleFunc("POST /api/healthchecks/{id}/actions", handleAPICreateAction(store))
	mux.HandleFunc("POST /api/healthchecks/{id}/generate-actions", handleAPIGenerateActions(store))
	mux.HandleFunc("POST /api/actions/{id}/complete", handleAPICompleteAction(store))
	mux.HandleFunc("POST /api/healthchecks/{id}/vote", handleAPIVote(store))
	mux.HandleFunc("POST /api/templates", handleAPICreateTemplate(store))
	mux.HandleFunc("POST /api/teams/{id}/healthchecks", handleAPICreateHealthCheck(store))

	// WebSocket
	mux.HandleFunc("GET /ws", hub.HandleWebSocket)

	// SPA — serve the embedded single-file React app
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		data, err := spaFS.ReadFile("spa/index.html")
		if err != nil {
			http.Error(w, "dashboard not available", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	return hub
}

// Mux returns the underlying ServeMux for plugin route registration.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// New creates a new dashboard server.
func New(cfg Config) *Server {
	mux := http.NewServeMux()
	hub := RegisterRoutes(mux, cfg.Store, cfg.Logger, cfg.Bus)

	// CORS middleware
	handler := corsMiddleware(mux)

	return &Server{
		mux: mux,
		httpSrv: &http.Server{
			Addr:    cfg.Addr,
			Handler: handler,
		},
		hub:    hub,
		logger: cfg.Logger,
	}
}

// Start starts the dashboard server. Blocks until the server stops.
func (s *Server) Start() error {
	go s.hub.Run()
	s.logger.Info().Str("addr", s.httpSrv.Addr).Msg("dashboard server started")
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the dashboard server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
