// Package jira provides a Jira integration plugin that syncs action items to Jira issues.
//
// Enable in config.yaml:
//
//	plugins:
//	  jira:
//	    enabled: true
//
// Environment variables:
//
//	JIRA_BASE_URL=https://your-domain.atlassian.net
//	JIRA_EMAIL=your-email@example.com
//	JIRA_API_TOKEN=your-api-token
//	JIRA_PROJECT_KEY=TEAM
package jira

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	jira "github.com/felixgeelhaar/jirasdk"
	"github.com/felixgeelhaar/jirasdk/core/issue"

	"github.com/felixgeelhaar/go-teamhealthcheck/sdk"
)

// JiraPlugin syncs health check action items to Jira issues.
type JiraPlugin struct {
	db         *sql.DB
	store      sdk.StoreReader
	logger     sdk.Logger
	client     *jira.Client
	projectKey string
}

func (p *JiraPlugin) Name() string        { return "jira" }
func (p *JiraPlugin) Version() string     { return "1.0.0" }
func (p *JiraPlugin) Description() string { return "Sync action items to Jira issues" }

func (p *JiraPlugin) Init(ctx sdk.PluginContext) error {
	p.db = ctx.DB
	p.store = ctx.Store
	p.logger = ctx.Logger

	baseURL := os.Getenv("JIRA_BASE_URL")
	email := os.Getenv("JIRA_EMAIL")
	apiToken := os.Getenv("JIRA_API_TOKEN")
	p.projectKey = os.Getenv("JIRA_PROJECT_KEY")

	if baseURL == "" || email == "" || apiToken == "" || p.projectKey == "" {
		return fmt.Errorf("jira plugin requires JIRA_BASE_URL, JIRA_EMAIL, JIRA_API_TOKEN, JIRA_PROJECT_KEY environment variables")
	}

	client, err := jira.NewClient(
		jira.WithBaseURL(baseURL),
		jira.WithAPIToken(email, apiToken),
		jira.WithTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create jira client: %w", err)
	}
	p.client = client

	return nil
}

func (p *JiraPlugin) Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jira_synced_actions (
			action_id  TEXT PRIMARY KEY,
			jira_key   TEXT NOT NULL,
			synced_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (p *JiraPlugin) RegisterRoutes(reg sdk.RouteRegistry) {
	reg.HandleFunc("POST /api/jira/sync-action/{actionId}", p.handleSyncAction)
	reg.HandleFunc("GET /api/jira/status/{actionId}", p.handleGetStatus)
}

func (p *JiraPlugin) UIManifest() []sdk.UIEntry {
	return []sdk.UIEntry{{
		Name:   "jira",
		Label:  "Jira",
		Icon:   "\U0001f4cb",
		Route:  "",
		NavPos: "healthcheck",
	}}
}

func (p *JiraPlugin) handleSyncAction(w http.ResponseWriter, r *http.Request) {
	actionID := r.PathValue("actionId")

	// Check if already synced
	var existingKey string
	err := p.db.QueryRow(`SELECT jira_key FROM jira_synced_actions WHERE action_id = ?`, actionID).Scan(&existingKey)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_synced", "jira_key": existingKey})
		return
	}

	// Get action details from request body
	var req struct {
		Description string `json:"description"`
		MetricName  string `json:"metric_name"`
		Assignee    string `json:"assignee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}

	summary := fmt.Sprintf("[Health Check] %s", req.MetricName)
	if req.MetricName == "" {
		summary = "[Health Check] Action Item"
	}

	// Create Jira issue
	created, err := p.client.Issue.Create(context.Background(), &issue.CreateInput{
		Fields: &issue.IssueFields{
			Project:     &issue.Project{Key: p.projectKey},
			Summary:     summary,
			Description: issue.ADFFromText(req.Description),
			IssueType:   &issue.IssueType{Name: "Task"},
		},
	})
	if err != nil {
		p.logger.Error().Err(err).Str("action_id", actionID).Msg("failed to create jira issue")
		http.Error(w, fmt.Sprintf("jira error: %v", err), 500)
		return
	}

	// Record the sync
	p.db.Exec(`INSERT INTO jira_synced_actions (action_id, jira_key) VALUES (?, ?)`,
		actionID, created.Key)

	p.logger.Info().Str("action_id", actionID).Str("jira_key", created.Key).Msg("action synced to jira")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "synced",
		"jira_key": created.Key,
	})
}

func (p *JiraPlugin) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	actionID := r.PathValue("actionId")

	var jiraKey string
	err := p.db.QueryRow(`SELECT jira_key FROM jira_synced_actions WHERE action_id = ?`, actionID).Scan(&jiraKey)
	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "not_synced"})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "synced", "jira_key": jiraKey})
}
