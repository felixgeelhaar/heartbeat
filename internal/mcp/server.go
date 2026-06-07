package mcp

import (
	"context"
	"fmt"

	bolt "go.klarlabs.de/bolt"
	"go.klarlabs.de/mcp"

	"github.com/felixgeelhaar/heartbeat/internal/domain"
	"github.com/felixgeelhaar/heartbeat/internal/mcpui"
	"github.com/felixgeelhaar/heartbeat/internal/storage"
)

// NewServer creates a fully configured MCP server with all health check tools registered.
func NewServer(store *storage.Store, logger *bolt.Logger, lc domain.HealthCheckLifecycle) *mcp.Server {
	srv := mcp.NewServer(mcp.ServerInfo{
		Name:    "heartbeat",
		Version: "1.0.0",
	})

	registerTeamTools(srv, store, logger)
	registerTemplateTools(srv, store, logger)
	registerHealthCheckTools(srv, store, logger, lc)
	registerVoteTools(srv, store, logger)
	registerCompareTools(srv, store, logger)
	registerAnalyzeTools(srv, store, logger)
	registerUIResources(srv, store)

	return srv
}

func registerUIResources(srv *mcp.Server, store *storage.Store) {
	srv.Resource("ui://healthcheck/{id}/vote").
		Name("Health Check Voting Form").
		Description("Interactive voting form for a health check session").
		MimeType("text/html;profile=mcp-app").
		Handler(func(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
			hcID := params["id"]
			hc, err := store.FindHealthCheckByID(hcID)
			if err != nil || hc == nil {
				return nil, fmt.Errorf("health check not found")
			}
			tmpl, err := store.FindTemplateByID(hc.TemplateID)
			if err != nil || tmpl == nil {
				return nil, fmt.Errorf("template not found")
			}
			return &mcp.ResourceContent{
				URI:      uri,
				MimeType: "text/html;profile=mcp-app",
				Text:     mcpui.VotingFormHTML(hcID, tmpl.Metrics),
			}, nil
		})

	srv.Resource("ui://healthcheck/{id}/results").
		Name("Health Check Results").
		Description("Visual results heatmap for a health check session").
		MimeType("text/html;profile=mcp-app").
		Handler(func(ctx context.Context, uri string, params map[string]string) (*mcp.ResourceContent, error) {
			hcID := params["id"]
			hc, err := store.FindHealthCheckByID(hcID)
			if err != nil || hc == nil {
				return nil, fmt.Errorf("health check not found")
			}
			tmpl, err := store.FindTemplateByID(hc.TemplateID)
			if err != nil || tmpl == nil {
				return nil, fmt.Errorf("template not found")
			}
			votes, err := store.FindVotesByHealthCheck(hcID)
			if err != nil {
				return nil, err
			}
			results := domain.ComputeMetricResults(votes, tmpl.Metrics)
			avgScore, _, _ := domain.ComputeOverallScore(results, votes)

			return &mcp.ResourceContent{
				URI:      uri,
				MimeType: "text/html;profile=mcp-app",
				Text:     mcpui.ResultsViewHTML(results, avgScore, len(votes)),
			}, nil
		})
}
