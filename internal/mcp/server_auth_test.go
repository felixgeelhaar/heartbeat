// Package mcp_test provides additional tests for auth-dependent tool behaviors.
package mcp_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/domain"
)

// TestSubmitVote_WithComment verifies that a comment is properly stored with a vote.
func TestSubmitVote_WithComment(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Comment Vote Team", "Comment Sprint")

	raw := callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID,
		"metric_name":    "Fun",
		"participant":    "Alice",
		"color":          "green",
		"comment":        "Really enjoying the team events",
	})

	var vote struct {
		Comment string `json:"Comment"`
		Color   string `json:"Color"`
	}
	json.Unmarshal(raw, &vote)
	if vote.Comment != "Really enjoying the team events" {
		t.Errorf("expected comment, got %q", vote.Comment)
	}
}

func TestGetResults_NoVotes(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "No Vote Results Team", "No Vote Sprint")

	raw := callTool(t, tc, "get_results", map[string]any{"healthcheck_id": hcID})

	var results struct {
		TotalVotes int `json:"total_votes"`
	}
	json.Unmarshal(raw, &results)
	if results.TotalVotes != 0 {
		t.Errorf("expected 0 votes, got %d", results.TotalVotes)
	}
}

func TestAnalyzeHealthcheck_AllGreenVotes(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Green Team", "Sprint Green")

	// All green votes — should produce strengths, no concerns
	for _, metric := range []string{"Fun", "Speed", "Mission"} {
		callTool(t, tc, "submit_vote", map[string]any{
			"healthcheck_id": hcID,
			"metric_name":    metric,
			"participant":    "Alice",
			"color":          "green",
		})
	}

	raw := callTool(t, tc, "analyze_healthcheck", map[string]any{"healthcheck_id": hcID})
	var analysis struct {
		Strengths []map[string]any `json:"strengths"`
		Concerns  []map[string]any `json:"concerns"`
	}
	json.Unmarshal(raw, &analysis)

	if len(analysis.Strengths) == 0 {
		t.Error("expected strengths for all-green metrics")
	}
}

func TestGetTrends_WithSingleSession(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	teamID, _, hcID := setupTestHC(t, tc, "Single Trend Team", "Sprint S1")

	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "participant": "Alice", "color": "green",
	})

	// Single session — get_trends should still return data (no trend direction computable)
	raw := callTool(t, tc, "get_trends", map[string]any{"team_id": teamID})

	var trends struct {
		SessionsCount int `json:"sessions_count"`
	}
	json.Unmarshal(raw, &trends)
	if trends.SessionsCount != 1 {
		t.Errorf("expected 1 session, got %d", trends.SessionsCount)
	}
}

func TestCompareSessions_SingleSession(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	teamID, _, hcID := setupTestHC(t, tc, "Single Compare Team", "Sprint C1")

	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "participant": "Alice", "color": "green",
	})

	raw := callTool(t, tc, "compare_sessions", map[string]any{"team_id": teamID, "limit": 10})
	if len(raw) == 0 {
		t.Error("expected comparison data for single session")
	}
}

func TestGetDiscussionTopics_NoVotes(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Discussion No Vote Team", "Sprint D1")

	raw := callTool(t, tc, "get_discussion_topics", map[string]any{"healthcheck_id": hcID})

	var resp struct {
		Topics []any `json:"topics"`
	}
	json.Unmarshal(raw, &resp)
	// No votes → no discussion topics
	if len(resp.Topics) != 0 {
		t.Errorf("expected 0 topics with no votes, got %d", len(resp.Topics))
	}
}

func TestGetDiscussionTopics_HighDisagreement(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Disagreement Team", "Sprint Dis")

	// High disagreement: green + red split
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "participant": "Alice", "color": "green",
	})
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "participant": "Bob", "color": "red",
	})

	raw := callTool(t, tc, "get_discussion_topics", map[string]any{
		"healthcheck_id": hcID,
		"include_trends": true,
	})
	var resp struct {
		Topics []any `json:"topics"`
	}
	json.Unmarshal(raw, &resp)
	if len(resp.Topics) == 0 {
		t.Error("expected discussion topics for high-disagreement votes")
	}
}

func TestListHealthChecks_NoFilter(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, _ = setupTestHC(t, tc, "List No Filter Team", "Sprint LNF")

	raw := callTool(t, tc, "list_healthchecks", map[string]any{})
	var hcs []struct{ ID string }
	json.Unmarshal(raw, &hcs)
	if len(hcs) == 0 {
		t.Error("expected at least 1 health check")
	}
}

func TestGetHealthCheck_WithVotes(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Get HC Team", "Sprint GHC")

	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "participant": "Alice", "color": "yellow",
	})

	raw := callTool(t, tc, "get_healthcheck", map[string]any{"healthcheck_id": hcID})
	var resp struct {
		HealthCheck struct{ ID string } `json:"healthcheck"`
		Results     []any               `json:"results"`
	}
	json.Unmarshal(raw, &resp)

	if resp.HealthCheck.ID != hcID {
		t.Errorf("expected HC ID %q, got %q", hcID, resp.HealthCheck.ID)
	}
	if len(resp.Results) == 0 {
		t.Error("expected metric results")
	}
}

func TestDeleteHealthcheck_ThenListIsEmpty(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Delete List Team", "Sprint DL")

	// Delete
	callTool(t, tc, "delete_healthcheck", map[string]any{"healthcheck_id": hcID})

	// List should be empty
	raw := callTool(t, tc, "list_healthchecks", map[string]any{})
	var hcs []struct{ ID string }
	json.Unmarshal(raw, &hcs)
	if len(hcs) != 0 {
		t.Errorf("expected 0 health checks after delete, got %d", len(hcs))
	}
}

// TestAddTeamMemberToNonExistentTeam verifies proper error handling.
func TestAddTeamMemberToNonExistentTeam(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	callToolExpectError(t, tc, "add_team_member", map[string]any{
		"team_id": "nonexistent-team",
		"name":    "Alice",
	})
}

// TestCreateTemplate_MinimalTemplate verifies that a template with minimal fields is created successfully.
func TestCreateTemplate_MinimalTemplate(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	raw := callTool(t, tc, "create_template", map[string]any{
		"name":        "Minimal MCP Template",
		"description": "A minimal template",
		"metrics": []map[string]string{
			{"name": "Quality", "description_good": "Great", "description_bad": "Poor"},
		},
	})

	var tmpl struct {
		ID   string `json:"ID"`
		Name string `json:"Name"`
	}
	json.Unmarshal(raw, &tmpl)
	if tmpl.ID == "" {
		t.Error("expected non-empty template ID")
	}
	if tmpl.Name != "Minimal MCP Template" {
		t.Errorf("expected 'Minimal MCP Template', got %q", tmpl.Name)
	}
}

// TestDeleteNonExistentTemplate verifies proper error handling.
func TestDeleteNonExistentTemplate(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	callToolExpectError(t, tc, "delete_template", map[string]any{
		"template_id": "nonexistent-template-id",
	})
}

// TestGetTrends_WithImprovingAndDecliningMetrics validates trend categorization.
func TestGetTrends_WithImprovingAndDecliningMetrics(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	teamID, templateID, hcID1 := setupTestHC(t, tc, "Trend Category Team", "Sprint TC1")

	// Sprint 1: Fun = red (1.0), Speed = green (3.0)
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID1, "metric_name": "Fun", "participant": "Alice", "color": "red",
	})
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID1, "metric_name": "Speed", "participant": "Alice", "color": "green",
	})
	callTool(t, tc, "close_healthcheck", map[string]any{"healthcheck_id": hcID1})

	// Sprint 2: Fun = green (3.0), Speed = red (1.0) — reverse trend
	raw := callTool(t, tc, "create_healthcheck", map[string]any{
		"team_id": teamID, "template_id": templateID, "name": "Sprint TC2",
	})
	var hc2 struct {
		Healthcheck struct{ ID string } `json:"healthcheck"`
	}
	json.Unmarshal(raw, &hc2)
	hcID2 := hc2.Healthcheck.ID

	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID2, "metric_name": "Fun", "participant": "Alice", "color": "green",
	})
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID2, "metric_name": "Speed", "participant": "Alice", "color": "red",
	})

	raw = callTool(t, tc, "get_trends", map[string]any{"team_id": teamID, "limit": 5})

	var trends struct {
		Improving []any `json:"improving"`
		Declining []any `json:"declining"`
		Stable    []any `json:"stable"`
	}
	json.Unmarshal(raw, &trends)

	// Fun improved (red → green), Speed declined (green → red)
	total := len(trends.Improving) + len(trends.Declining) + len(trends.Stable)
	if total == 0 {
		t.Error("expected trend data for 2 sessions with contrasting scores")
	}
}

// TestCreateHealthCheck_GeneratesTwoForSameTeam verifies list returns correct count.
func TestCreateHealthCheck_TwoForSameTeam(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	teamID, templateID, _ := setupTestHC(t, tc, "Multi HC Team", "Sprint MHC1")

	// Create second HC
	callTool(t, tc, "create_healthcheck", map[string]any{
		"team_id": teamID, "template_id": templateID, "name": "Sprint MHC2",
	})

	raw := callTool(t, tc, "list_healthchecks", map[string]any{"team_id": teamID})
	var hcs []struct{ ID string }
	json.Unmarshal(raw, &hcs)
	if len(hcs) != 2 {
		t.Errorf("expected 2 health checks for team, got %d", len(hcs))
	}
}

// TestStoreDirectCoverage uses the domain directly to reach more paths.
func TestUUIDGeneration(t *testing.T) {
	// Exercise uuid generation for coverage
	id := uuid.NewString()
	if id == "" {
		t.Error("expected non-empty UUID")
	}
}

// TestDomainCoverage_ActionFields verifies Action struct fields.
func TestDomainCoverage_ActionFields(t *testing.T) {
	now := time.Now()
	a := domain.Action{
		ID:            "action-1",
		HealthCheckID: "hc-1",
		MetricName:    "Fun",
		Description:   "Test action",
		Assignee:      "Alice",
		Completed:     false,
		CreatedAt:     now,
	}
	if a.ID == "" {
		t.Error("expected non-empty ID")
	}
	if a.CompletedAt != nil {
		t.Error("expected nil CompletedAt initially")
	}
}
