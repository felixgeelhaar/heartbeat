// Package linear provides a Linear integration plugin that syncs action items to Linear issues.
//
// Enable in config.yaml:
//
//	plugins:
//	  linear:
//	    enabled: true
//
// Environment variables:
//
//	LINEAR_API_KEY=lin_api_xxxxx
//	LINEAR_TEAM_ID=your-team-uuid
package linear

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/felixgeelhaar/go-teamhealthcheck/sdk"
)

// LinearPlugin syncs health check action items to Linear issues.
type LinearPlugin struct {
	db     *sql.DB
	store  sdk.StoreReader
	logger sdk.Logger
	apiKey string
	teamID string
	client *http.Client
}

func (p *LinearPlugin) Name() string        { return "linear" }
func (p *LinearPlugin) Version() string     { return "1.0.0" }
func (p *LinearPlugin) Description() string { return "Sync action items to Linear issues" }

func (p *LinearPlugin) Init(ctx sdk.PluginContext) error {
	p.db = ctx.DB
	p.store = ctx.Store
	p.logger = ctx.Logger

	p.apiKey = os.Getenv("LINEAR_API_KEY")
	p.teamID = os.Getenv("LINEAR_TEAM_ID")

	if p.apiKey == "" || p.teamID == "" {
		return fmt.Errorf("linear plugin requires LINEAR_API_KEY and LINEAR_TEAM_ID environment variables")
	}

	p.client = &http.Client{Timeout: 30 * time.Second}
	return nil
}

func (p *LinearPlugin) Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS linear_synced_actions (
			action_id    TEXT PRIMARY KEY,
			linear_id    TEXT NOT NULL,
			linear_url   TEXT NOT NULL DEFAULT '',
			synced_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (p *LinearPlugin) RegisterRoutes(reg sdk.RouteRegistry) {
	reg.HandleFunc("POST /api/linear/sync-action/{actionId}", p.handleSyncAction)
	reg.HandleFunc("GET /api/linear/status/{actionId}", p.handleGetStatus)
}

func (p *LinearPlugin) UIManifest() []sdk.UIEntry {
	return []sdk.UIEntry{{
		Name:   "linear",
		Label:  "Linear",
		Icon:   "\U0001f4dd",
		Route:  "",
		NavPos: "healthcheck",
	}}
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphqlResponse struct {
	Data struct {
		IssueCreate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				URL        string `json:"url"`
			} `json:"issue"`
		} `json:"issueCreate"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (p *LinearPlugin) handleSyncAction(w http.ResponseWriter, r *http.Request) {
	actionID := r.PathValue("actionId")

	// Check if already synced
	var existingID, existingURL string
	err := p.db.QueryRow(`SELECT linear_id, linear_url FROM linear_synced_actions WHERE action_id = ?`, actionID).Scan(&existingID, &existingURL)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_synced", "linear_id": existingID, "linear_url": existingURL})
		return
	}

	var req struct {
		Description string `json:"description"`
		MetricName  string `json:"metric_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}

	title := fmt.Sprintf("[Health Check] %s", req.MetricName)
	if req.MetricName == "" {
		title = "[Health Check] Action Item"
	}

	// Create Linear issue via GraphQL API
	gql := graphqlRequest{
		Query: `mutation IssueCreate($input: IssueCreateInput!) {
			issueCreate(input: $input) {
				success
				issue { id identifier url }
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{
				"teamId":      p.teamID,
				"title":       title,
				"description": req.Description,
			},
		},
	}

	body, _ := json.Marshal(gql)
	httpReq, _ := http.NewRequest("POST", "https://api.linear.app/graphql", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		p.logger.Error().Err(err).Str("action_id", actionID).Msg("failed to call linear api")
		http.Error(w, fmt.Sprintf("linear error: %v", err), 500)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var gqlResp graphqlResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		http.Error(w, "failed to parse linear response", 500)
		return
	}

	if len(gqlResp.Errors) > 0 {
		http.Error(w, fmt.Sprintf("linear error: %s", gqlResp.Errors[0].Message), 500)
		return
	}

	if !gqlResp.Data.IssueCreate.Success {
		http.Error(w, "linear issue creation failed", 500)
		return
	}

	linearIssue := gqlResp.Data.IssueCreate.Issue
	p.db.Exec(`INSERT INTO linear_synced_actions (action_id, linear_id, linear_url) VALUES (?, ?, ?)`,
		actionID, linearIssue.ID, linearIssue.URL)

	p.logger.Info().Str("action_id", actionID).Str("linear_id", linearIssue.Identifier).Msg("action synced to linear")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "synced",
		"linear_id":  linearIssue.Identifier,
		"linear_url": linearIssue.URL,
	})
}

func (p *LinearPlugin) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	actionID := r.PathValue("actionId")

	var linearID, linearURL string
	err := p.db.QueryRow(`SELECT linear_id, linear_url FROM linear_synced_actions WHERE action_id = ?`, actionID).Scan(&linearID, &linearURL)
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
	json.NewEncoder(w).Encode(map[string]string{"status": "synced", "linear_id": linearID, "linear_url": linearURL})
}
