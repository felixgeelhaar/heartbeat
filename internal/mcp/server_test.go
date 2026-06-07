package mcp_test

import (
	"encoding/json"
	"os"
	"testing"

	bolt "go.klarlabs.de/bolt"
	"go.klarlabs.de/mcp/testutil"

	"github.com/felixgeelhaar/heartbeat/internal/lifecycle"
	mcptools "github.com/felixgeelhaar/heartbeat/internal/mcp"
	"github.com/felixgeelhaar/heartbeat/internal/storage"
)

func newTestServer(t *testing.T) *testutil.TestClient {
	t.Helper()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, err := storage.New(":memory:", logger)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	lc, err := lifecycle.New(logger)
	if err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	srv := mcptools.NewServer(store, logger, lc)
	return testutil.NewTestClient(t, srv)
}

// callTool calls a tool and returns the parsed JSON result from the text content.
func callTool(t *testing.T, tc *testutil.TestClient, name string, args map[string]any) json.RawMessage {
	t.Helper()

	resp, err := tc.CallToolRaw(name, args)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if resp.Error != nil {
		t.Fatalf("call %s error: %s", name, resp.Error.Message)
	}

	// The result is the raw handler return value serialized by the testutil.
	// Extract it from the content array.
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}

	content, ok := result["content"]
	if !ok {
		t.Fatalf("no content in response")
	}

	// content is []map[string]any with {"type":"text","text":...}
	var contentItems []map[string]any
	switch v := content.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				contentItems = append(contentItems, m)
			}
		}
	case []map[string]any:
		contentItems = v
	}

	if len(contentItems) == 0 {
		t.Fatalf("empty content for %s", name)
	}

	// The "text" field contains the handler's return value.
	// For struct/map returns, it may be the raw Go value or a JSON string.
	textVal := contentItems[0]["text"]

	// If it's already a string (JSON), return as-is
	if s, ok := textVal.(string); ok {
		return json.RawMessage(s)
	}

	// Otherwise marshal the raw value
	data, err := json.Marshal(textVal)
	if err != nil {
		t.Fatalf("marshal text value for %s: %v", name, err)
	}
	return data
}

// callToolExpectError calls a tool and expects it to return an error.
func callToolExpectError(t *testing.T, tc *testutil.TestClient, name string, args map[string]any) {
	t.Helper()

	resp, err := tc.CallToolRaw(name, args)
	if err != nil {
		return // Error at transport level is fine
	}
	if resp.Error != nil {
		return // Error in response is fine
	}
	t.Errorf("expected error from %s, got success", name)
}

func TestAllToolsRegistered(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	tools, err := tc.ListTools()
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	expected := []string{
		"create_team", "list_teams", "get_team", "delete_team",
		"add_team_member", "remove_team_member",
		"list_templates", "get_template", "create_template", "delete_template",
		"create_healthcheck", "list_healthchecks", "get_healthcheck",
		"close_healthcheck", "reopen_healthcheck", "archive_healthcheck",
		"delete_healthcheck", "my_pending_healthchecks",
		"submit_vote", "get_results",
		"compare_sessions",
		"analyze_healthcheck", "get_trends", "get_discussion_topics",
	}

	registered := make(map[string]bool)
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		registered[name] = true
	}

	for _, name := range expected {
		if !registered[name] {
			t.Errorf("tool %q not registered", name)
		}
	}

	if len(tools) != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), len(tools))
	}
}

func TestEndToEndHealthCheckFlow(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	// 1. List templates to get the Spotify template ID
	raw := callTool(t, tc, "list_templates", map[string]any{})
	var templates []struct {
		ID   string `json:"ID"`
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(raw, &templates); err != nil {
		t.Fatalf("unmarshal templates: %v\nraw: %s", err, raw)
	}
	if len(templates) == 0 {
		t.Fatal("no templates found")
	}
	var templateID string
	for _, tmpl := range templates {
		if tmpl.Name == "Spotify Squad Health Check" {
			templateID = tmpl.ID
			break
		}
	}
	if templateID == "" {
		templateID = templates[0].ID
	}

	// 2. Create a team
	raw = callTool(t, tc, "create_team", map[string]any{
		"name":    "Platform",
		"members": []string{"Alice", "Bob", "Carol"},
	})
	var team struct {
		ID   string `json:"ID"`
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(raw, &team); err != nil {
		t.Fatalf("unmarshal team: %v\nraw: %s", err, raw)
	}
	if team.Name != "Platform" {
		t.Errorf("expected team name 'Platform', got %q", team.Name)
	}

	// 3. Create a health check session
	raw = callTool(t, tc, "create_healthcheck", map[string]any{
		"team_id":     team.ID,
		"template_id": templateID,
		"name":        "Sprint 42",
	})
	var hcResp struct {
		Healthcheck struct {
			ID string `json:"ID"`
		} `json:"healthcheck"`
	}
	if err := json.Unmarshal(raw, &hcResp); err != nil {
		t.Fatalf("unmarshal healthcheck: %v\nraw: %s", err, raw)
	}
	hcID := hcResp.Healthcheck.ID

	// 4. Submit votes
	votes := []struct {
		participant string
		metric      string
		color       string
	}{
		{"Alice", "Fun", "green"},
		{"Bob", "Fun", "green"},
		{"Carol", "Fun", "yellow"},
		{"Alice", "Tech Quality", "red"},
		{"Bob", "Tech Quality", "yellow"},
		{"Carol", "Tech Quality", "red"},
	}
	for _, v := range votes {
		callTool(t, tc, "submit_vote", map[string]any{
			"healthcheck_id": hcID,
			"metric_name":    v.metric,
			"participant":    v.participant,
			"color":          v.color,
		})
	}

	// 5. Get results
	raw = callTool(t, tc, "get_results", map[string]any{
		"healthcheck_id": hcID,
	})
	var results struct {
		TotalVotes   int `json:"total_votes"`
		Participants int `json:"participants"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("unmarshal results: %v\nraw: %s", err, raw)
	}
	if results.TotalVotes != 6 {
		t.Errorf("expected 6 total votes, got %d", results.TotalVotes)
	}
	if results.Participants != 3 {
		t.Errorf("expected 3 participants, got %d", results.Participants)
	}

	// 6. Analyze
	callTool(t, tc, "analyze_healthcheck", map[string]any{
		"healthcheck_id": hcID,
	})

	// 7. Get discussion topics
	callTool(t, tc, "get_discussion_topics", map[string]any{
		"healthcheck_id": hcID,
	})

	// 8. Close the health check
	callTool(t, tc, "close_healthcheck", map[string]any{
		"healthcheck_id": hcID,
	})

	// 9. Voting on closed session should fail
	callToolExpectError(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID,
		"metric_name":    "Fun",
		"participant":    "Alice",
		"color":          "red",
	})
}

func TestInvalidVoteColor(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	// Create team and healthcheck
	raw := callTool(t, tc, "list_templates", map[string]any{})
	var templates []struct{ ID string }
	json.Unmarshal(raw, &templates)
	if len(templates) == 0 {
		t.Fatal("no templates")
	}

	raw = callTool(t, tc, "create_team", map[string]any{"name": "T1"})
	var team struct{ ID string }
	json.Unmarshal(raw, &team)

	raw = callTool(t, tc, "create_healthcheck", map[string]any{
		"team_id": team.ID, "template_id": templates[0].ID, "name": "S1",
	})
	var hc struct {
		Healthcheck struct{ ID string } `json:"healthcheck"`
	}
	json.Unmarshal(raw, &hc)

	callToolExpectError(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hc.Healthcheck.ID,
		"metric_name":    "Fun",
		"participant":    "Alice",
		"color":          "blue",
	})
}

// setupTestHC creates a team + healthcheck and returns their IDs and the template ID.
func setupTestHC(t *testing.T, tc *testutil.TestClient, teamName, hcName string) (teamID, templateID, hcID string) {
	t.Helper()
	raw := callTool(t, tc, "list_templates", map[string]any{})
	var templates []struct {
		ID   string
		Name string
	}
	json.Unmarshal(raw, &templates)
	// Find the Spotify template
	for _, tmpl := range templates {
		if tmpl.Name == "Spotify Squad Health Check" {
			templateID = tmpl.ID
			break
		}
	}
	if templateID == "" {
		templateID = templates[0].ID
	}

	raw = callTool(t, tc, "create_team", map[string]any{"name": teamName, "members": []string{"Alice", "Bob"}})
	var team struct{ ID string }
	json.Unmarshal(raw, &team)
	teamID = team.ID

	raw = callTool(t, tc, "create_healthcheck", map[string]any{
		"team_id": teamID, "template_id": templateID, "name": hcName,
	})
	var hcResp struct {
		Healthcheck struct{ ID string } `json:"healthcheck"`
	}
	json.Unmarshal(raw, &hcResp)
	hcID = hcResp.Healthcheck.ID
	return
}

func TestTeamTools(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	// Create team
	raw := callTool(t, tc, "create_team", map[string]any{"name": "TeamA", "members": []string{"Alice"}})
	var team struct{ ID, Name string }
	json.Unmarshal(raw, &team)
	if team.Name != "TeamA" {
		t.Errorf("expected TeamA, got %s", team.Name)
	}

	// List teams
	raw = callTool(t, tc, "list_teams", map[string]any{})
	var teams []struct{ ID, Name string }
	json.Unmarshal(raw, &teams)
	if len(teams) != 1 {
		t.Errorf("expected 1 team, got %d", len(teams))
	}

	// Get team
	raw = callTool(t, tc, "get_team", map[string]any{"team_id": team.ID})
	var gotTeam struct {
		ID      string
		Name    string
		Members []string
	}
	json.Unmarshal(raw, &gotTeam)
	if len(gotTeam.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(gotTeam.Members))
	}

	// Add member
	callTool(t, tc, "add_team_member", map[string]any{"team_id": team.ID, "name": "Bob"})

	// Remove member
	callTool(t, tc, "remove_team_member", map[string]any{"team_id": team.ID, "name": "Alice"})

	// Get team not found
	callToolExpectError(t, tc, "get_team", map[string]any{"team_id": "nonexistent"})

	// Delete team
	callTool(t, tc, "delete_team", map[string]any{"team_id": team.ID})
}

func TestTemplateTools(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	// List templates (should have all built-in templates)
	raw := callTool(t, tc, "list_templates", map[string]any{})
	var templates []struct{ ID, Name string }
	json.Unmarshal(raw, &templates)
	if len(templates) < 4 {
		t.Errorf("expected at least 4 built-in templates, got %d", len(templates))
	}

	// Find and get Spotify template
	var spotifyID string
	for _, tmpl := range templates {
		if tmpl.Name == "Spotify Squad Health Check" {
			spotifyID = tmpl.ID
			break
		}
	}
	if spotifyID == "" {
		t.Fatal("Spotify template not found")
	}
	raw = callTool(t, tc, "get_template", map[string]any{"template_id": spotifyID})
	var tmpl struct {
		ID      string
		Metrics []struct{ Name string }
	}
	json.Unmarshal(raw, &tmpl)
	if len(tmpl.Metrics) != 10 {
		t.Errorf("expected 10 metrics, got %d", len(tmpl.Metrics))
	}

	// Get template not found
	callToolExpectError(t, tc, "get_template", map[string]any{"template_id": "nonexistent"})

	// Create custom template
	raw = callTool(t, tc, "create_template", map[string]any{
		"name":        "Custom",
		"description": "Test",
		"metrics": []map[string]string{
			{"name": "M1", "description_good": "Good", "description_bad": "Bad"},
		},
	})
	var custom struct{ ID string }
	json.Unmarshal(raw, &custom)

	// Delete custom template
	callTool(t, tc, "delete_template", map[string]any{"template_id": custom.ID})

	// Delete built-in should fail
	callToolExpectError(t, tc, "delete_template", map[string]any{"template_id": templates[0].ID})
}

func TestHealthCheckLifecycleTools(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	teamID, _, hcID := setupTestHC(t, tc, "Lifecycle Team", "Sprint 1")

	// List health checks
	raw := callTool(t, tc, "list_healthchecks", map[string]any{})
	var hcs []struct{ ID string }
	json.Unmarshal(raw, &hcs)
	if len(hcs) != 1 {
		t.Errorf("expected 1 healthcheck, got %d", len(hcs))
	}

	// List with team filter
	raw = callTool(t, tc, "list_healthchecks", map[string]any{"team_id": teamID, "status": "open"})
	json.Unmarshal(raw, &hcs)
	if len(hcs) != 1 {
		t.Errorf("expected 1 open for team, got %d", len(hcs))
	}

	// Get healthcheck
	callTool(t, tc, "get_healthcheck", map[string]any{"healthcheck_id": hcID})

	// Get nonexistent
	callToolExpectError(t, tc, "get_healthcheck", map[string]any{"healthcheck_id": "nonexistent"})

	// Close without votes should fail (guard)
	callToolExpectError(t, tc, "close_healthcheck", map[string]any{"healthcheck_id": hcID})

	// Submit a vote then close
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "participant": "Alice", "color": "green",
	})
	callTool(t, tc, "close_healthcheck", map[string]any{"healthcheck_id": hcID})

	// Reopen
	callTool(t, tc, "reopen_healthcheck", map[string]any{"healthcheck_id": hcID})

	// Close again
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Speed", "participant": "Bob", "color": "yellow",
	})
	callTool(t, tc, "close_healthcheck", map[string]any{"healthcheck_id": hcID})

	// Archive
	callTool(t, tc, "archive_healthcheck", map[string]any{"healthcheck_id": hcID})

	// Archive nonexistent
	callToolExpectError(t, tc, "archive_healthcheck", map[string]any{"healthcheck_id": "nonexistent"})

	// Delete
	_, templateID, hcID2 := setupTestHC(t, tc, "Del Team", "Sprint Del")
	_ = templateID
	callTool(t, tc, "delete_healthcheck", map[string]any{"healthcheck_id": hcID2})
}

func TestCompareAndTrends(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	teamID, templateID, hcID1 := setupTestHC(t, tc, "Trend Team", "Sprint 1")

	// Submit votes for sprint 1
	for _, metric := range []string{"Fun", "Speed"} {
		callTool(t, tc, "submit_vote", map[string]any{
			"healthcheck_id": hcID1, "metric_name": metric, "participant": "Alice", "color": "green",
		})
	}
	callTool(t, tc, "close_healthcheck", map[string]any{"healthcheck_id": hcID1})

	// Create sprint 2
	raw := callTool(t, tc, "create_healthcheck", map[string]any{
		"team_id": teamID, "template_id": templateID, "name": "Sprint 2",
	})
	var hc2 struct {
		Healthcheck struct{ ID string } `json:"healthcheck"`
	}
	json.Unmarshal(raw, &hc2)
	hcID2 := hc2.Healthcheck.ID

	for _, metric := range []string{"Fun", "Speed"} {
		callTool(t, tc, "submit_vote", map[string]any{
			"healthcheck_id": hcID2, "metric_name": metric, "participant": "Alice", "color": "red",
		})
	}

	// Compare sessions
	raw = callTool(t, tc, "compare_sessions", map[string]any{"team_id": teamID, "limit": 5})
	if len(raw) == 0 {
		t.Error("expected comparison data")
	}

	// Get trends
	raw = callTool(t, tc, "get_trends", map[string]any{"team_id": teamID})
	if len(raw) == 0 {
		t.Error("expected trend data")
	}

	// Compare with no sessions for wrong team
	callToolExpectError(t, tc, "compare_sessions", map[string]any{"team_id": "nonexistent"})
	callToolExpectError(t, tc, "get_trends", map[string]any{"team_id": "nonexistent"})
}

func TestVoteValidation(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Vote Team", "Sprint V")

	// Vote on nonexistent metric
	callToolExpectError(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Nonexistent", "participant": "Alice", "color": "green",
	})

	// Vote on nonexistent healthcheck
	callToolExpectError(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": "nonexistent", "metric_name": "Fun", "participant": "Alice", "color": "green",
	})

	// Get results for nonexistent
	callToolExpectError(t, tc, "get_results", map[string]any{"healthcheck_id": "nonexistent"})

	// Vote without participant (no auth context) should fail
	callToolExpectError(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "color": "green",
	})
}

func TestCreateHealthCheckValidation(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	// Nonexistent team
	raw := callTool(t, tc, "list_templates", map[string]any{})
	var templates []struct{ ID string }
	json.Unmarshal(raw, &templates)

	callToolExpectError(t, tc, "create_healthcheck", map[string]any{
		"team_id": "nonexistent", "template_id": templates[0].ID, "name": "S1",
	})

	// Nonexistent template
	raw = callTool(t, tc, "create_team", map[string]any{"name": "VTeam"})
	var team struct{ ID string }
	json.Unmarshal(raw, &team)

	callToolExpectError(t, tc, "create_healthcheck", map[string]any{
		"team_id": team.ID, "template_id": "nonexistent", "name": "S1",
	})
}

func TestCloseReopenNonexistent(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	callToolExpectError(t, tc, "close_healthcheck", map[string]any{"healthcheck_id": "nonexistent"})
	callToolExpectError(t, tc, "reopen_healthcheck", map[string]any{"healthcheck_id": "nonexistent"})
	callToolExpectError(t, tc, "delete_healthcheck", map[string]any{"healthcheck_id": "nonexistent"})
}

func TestAnalyzeNonexistent(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	callToolExpectError(t, tc, "analyze_healthcheck", map[string]any{"healthcheck_id": "nonexistent"})
	callToolExpectError(t, tc, "get_discussion_topics", map[string]any{"healthcheck_id": "nonexistent"})
}

func TestAnalyzeWithMixedVotes(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Analyze Team", "Sprint A")

	// Submit mixed votes to trigger disagreement detection
	votes := []struct {
		participant, metric, color string
	}{
		{"Alice", "Fun", "green"},
		{"Bob", "Fun", "red"},
		{"Alice", "Tech Quality", "red"},
		{"Bob", "Tech Quality", "red"},
		{"Alice", "Speed", "green"},
		{"Bob", "Speed", "green"},
	}
	for _, v := range votes {
		callTool(t, tc, "submit_vote", map[string]any{
			"healthcheck_id": hcID, "metric_name": v.metric, "participant": v.participant, "color": v.color,
		})
	}

	// Analyze should show strengths and concerns
	raw := callTool(t, tc, "analyze_healthcheck", map[string]any{"healthcheck_id": hcID})
	var analysis struct {
		Strengths []map[string]any `json:"strengths"`
		Concerns  []map[string]any `json:"concerns"`
	}
	json.Unmarshal(raw, &analysis)

	// Tech Quality (all red) should be a concern
	if len(analysis.Concerns) == 0 {
		t.Error("expected concerns for all-red metrics")
	}

	// Discussion topics with trends
	raw = callTool(t, tc, "get_discussion_topics", map[string]any{
		"healthcheck_id": hcID, "include_trends": true,
	})
	var topics struct {
		Topics []struct{ Metric string } `json:"topics"`
	}
	json.Unmarshal(raw, &topics)
	if len(topics.Topics) == 0 {
		t.Error("expected discussion topics for mixed/low-score metrics")
	}
}

func TestMyPendingHealthchecks_NoAuth(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	// Without auth context, should fail
	callToolExpectError(t, tc, "my_pending_healthchecks", map[string]any{})
}

func TestListHealthchecksFilters(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	teamID, _, hcID := setupTestHC(t, tc, "Filter Team2", "Sprint F1")

	// Submit vote and close first HC
	callTool(t, tc, "submit_vote", map[string]any{
		"healthcheck_id": hcID, "metric_name": "Fun", "participant": "Alice", "color": "green",
	})
	callTool(t, tc, "close_healthcheck", map[string]any{"healthcheck_id": hcID})

	// List with status filter
	raw := callTool(t, tc, "list_healthchecks", map[string]any{"status": "closed"})
	var closed []struct{ ID string }
	json.Unmarshal(raw, &closed)
	if len(closed) != 1 {
		t.Errorf("expected 1 closed, got %d", len(closed))
	}

	// List with team + status
	raw = callTool(t, tc, "list_healthchecks", map[string]any{"team_id": teamID, "status": "open"})
	var open []struct{ ID string }
	json.Unmarshal(raw, &open)
	if len(open) != 0 {
		t.Errorf("expected 0 open after closing, got %d", len(open))
	}

	// List with limit
	raw = callTool(t, tc, "list_healthchecks", map[string]any{"limit": 1})
	var limited []struct{ ID string }
	json.Unmarshal(raw, &limited)
	if len(limited) != 1 {
		t.Errorf("expected 1 with limit, got %d", len(limited))
	}
}

func TestReopenOpenHealthCheck(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Reopen Team", "Sprint R")

	// Reopen an already-open HC should fail
	callToolExpectError(t, tc, "reopen_healthcheck", map[string]any{"healthcheck_id": hcID})
}

func TestArchiveOpenHealthCheck(t *testing.T) {
	tc := newTestServer(t)
	defer tc.Close()

	_, _, hcID := setupTestHC(t, tc, "Archive Team", "Sprint Ar")

	// Archive an open HC should fail (must close first)
	callToolExpectError(t, tc, "archive_healthcheck", map[string]any{"healthcheck_id": hcID})
}
