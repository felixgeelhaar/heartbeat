package storage_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	bolt "go.klarlabs.de/bolt"

	"github.com/felixgeelhaar/heartbeat/internal/domain"
	"github.com/felixgeelhaar/heartbeat/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, err := storage.New(":memory:", logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSpotifyTemplateSeed(t *testing.T) {
	store := newTestStore(t)

	tmpl, err := store.FindTemplateByName("Spotify Squad Health Check")
	if err != nil {
		t.Fatalf("find template: %v", err)
	}
	if tmpl == nil {
		t.Fatal("spotify template not seeded")
	}
	if !tmpl.BuiltIn {
		t.Error("expected built_in=true")
	}
	if len(tmpl.Metrics) != 10 {
		t.Errorf("expected 10 metrics, got %d", len(tmpl.Metrics))
	}
}

func TestTeamCRUD(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	team := &domain.Team{
		ID:        uuid.NewString(),
		Name:      "Platform Team",
		Members:   []string{"Alice", "Bob"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.CreateTeam(team); err != nil {
		t.Fatalf("create team: %v", err)
	}

	got, err := store.FindTeamByID(team.ID)
	if err != nil {
		t.Fatalf("find team: %v", err)
	}
	if got.Name != "Platform Team" {
		t.Errorf("expected name %q, got %q", "Platform Team", got.Name)
	}
	if len(got.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(got.Members))
	}

	// Add member
	if err := store.AddTeamMember(team.ID, "Carol"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	got, _ = store.FindTeamByID(team.ID)
	if len(got.Members) != 3 {
		t.Errorf("expected 3 members, got %d", len(got.Members))
	}

	// Remove member
	if err := store.RemoveTeamMember(team.ID, "Bob"); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	got, _ = store.FindTeamByID(team.ID)
	if len(got.Members) != 2 {
		t.Errorf("expected 2 members after removal, got %d", len(got.Members))
	}

	// Delete team
	if err := store.DeleteTeam(team.ID); err != nil {
		t.Fatalf("delete team: %v", err)
	}
	got, _ = store.FindTeamByID(team.ID)
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestFullHealthCheckFlow(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	// Get seeded Spotify template
	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	if tmpl == nil {
		t.Fatal("spotify template not found")
	}

	// Create team
	team := &domain.Team{
		ID:        uuid.NewString(),
		Name:      "Test Team",
		Members:   []string{"Alice", "Bob", "Carol"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateTeam(team); err != nil {
		t.Fatalf("create team: %v", err)
	}

	// Create health check
	hc := &domain.HealthCheck{
		ID:         uuid.NewString(),
		TeamID:     team.ID,
		TemplateID: tmpl.ID,
		Name:       "Sprint 42",
		Status:     domain.StatusOpen,
		CreatedAt:  now,
	}
	if err := store.CreateHealthCheck(hc); err != nil {
		t.Fatalf("create healthcheck: %v", err)
	}

	// Submit votes
	votes := []struct {
		participant string
		metric      string
		color       domain.VoteColor
	}{
		{"Alice", "Fun", domain.VoteGreen},
		{"Bob", "Fun", domain.VoteGreen},
		{"Carol", "Fun", domain.VoteYellow},
		{"Alice", "Tech Quality", domain.VoteRed},
		{"Bob", "Tech Quality", domain.VoteYellow},
		{"Carol", "Tech Quality", domain.VoteRed},
	}
	for _, v := range votes {
		vote := &domain.Vote{
			ID:            uuid.NewString(),
			HealthCheckID: hc.ID,
			MetricName:    v.metric,
			Participant:   v.participant,
			Color:         v.color,
			CreatedAt:     now,
		}
		if err := store.UpsertVote(vote); err != nil {
			t.Fatalf("upsert vote: %v", err)
		}
	}

	// Verify results
	allVotes, err := store.FindVotesByHealthCheck(hc.ID)
	if err != nil {
		t.Fatalf("find votes: %v", err)
	}
	if len(allVotes) != 6 {
		t.Errorf("expected 6 votes, got %d", len(allVotes))
	}

	results := domain.ComputeMetricResults(allVotes, tmpl.Metrics)

	var funResult, techResult *domain.MetricResult
	for i := range results {
		switch results[i].MetricName {
		case "Fun":
			funResult = &results[i]
		case "Tech Quality":
			techResult = &results[i]
		}
	}

	if funResult == nil || funResult.GreenCount != 2 || funResult.YellowCount != 1 {
		t.Errorf("unexpected Fun result: %+v", funResult)
	}
	if techResult == nil || techResult.RedCount != 2 || techResult.YellowCount != 1 {
		t.Errorf("unexpected Tech Quality result: %+v", techResult)
	}

	// Test upsert (re-vote)
	reVote := &domain.Vote{
		ID:            uuid.NewString(),
		HealthCheckID: hc.ID,
		MetricName:    "Fun",
		Participant:   "Carol",
		Color:         domain.VoteGreen,
		CreatedAt:     now,
	}
	if err := store.UpsertVote(reVote); err != nil {
		t.Fatalf("upsert re-vote: %v", err)
	}
	allVotes, _ = store.FindVotesByHealthCheck(hc.ID)
	if len(allVotes) != 6 {
		t.Errorf("expected still 6 votes after upsert, got %d", len(allVotes))
	}

	// Close health check via direct status update
	hc.Status = domain.StatusClosed
	if err := store.UpdateHealthCheck(hc); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := store.FindHealthCheckByID(hc.ID)
	if got.Status != domain.StatusClosed {
		t.Errorf("expected closed, got %s", got.Status)
	}
}

func TestDeleteBuiltInTemplateBlocked(t *testing.T) {
	store := newTestStore(t)

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	if tmpl == nil {
		t.Fatal("spotify template not found")
	}

	err := store.DeleteTemplate(tmpl.ID)
	if err == nil {
		t.Error("expected error when deleting built-in template")
	}
}

func TestFindAllTeams(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	// No teams initially
	teams, err := store.FindAllTeams()
	if err != nil {
		t.Fatalf("find all teams: %v", err)
	}
	if len(teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(teams))
	}

	// Create two teams
	for _, name := range []string{"Alpha", "Beta"} {
		store.CreateTeam(&domain.Team{
			ID: uuid.NewString(), Name: name, Members: []string{"M1"},
			CreatedAt: now, UpdatedAt: now,
		})
	}

	teams, err = store.FindAllTeams()
	if err != nil {
		t.Fatalf("find all teams: %v", err)
	}
	if len(teams) != 2 {
		t.Errorf("expected 2 teams, got %d", len(teams))
	}
}

func TestFindAllTemplates(t *testing.T) {
	store := newTestStore(t)

	templates, err := store.FindAllTemplates()
	if err != nil {
		t.Fatalf("find all templates: %v", err)
	}
	// Should have at least the Spotify template
	if len(templates) < 1 {
		t.Error("expected at least 1 template (Spotify)")
	}
}

func TestFindTemplateByID(t *testing.T) {
	store := newTestStore(t)

	// Get by name first to get the ID
	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	if tmpl == nil {
		t.Fatal("spotify template not found")
	}

	// Now find by ID
	got, err := store.FindTemplateByID(tmpl.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got == nil {
		t.Fatal("expected template, got nil")
	}
	if got.Name != "Spotify Squad Health Check" {
		t.Errorf("expected Spotify template, got %s", got.Name)
	}
	if len(got.Metrics) != 10 {
		t.Errorf("expected 10 metrics, got %d", len(got.Metrics))
	}

	// Non-existent ID
	got, err = store.FindTemplateByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestCustomTemplateCRUD(t *testing.T) {
	store := newTestStore(t)

	tmpl := &domain.Template{
		ID:          uuid.NewString(),
		Name:        "Custom Template",
		Description: "Test template",
		BuiltIn:     false,
		Metrics: []domain.TemplateMetric{
			{ID: uuid.NewString(), Name: "Metric A", DescriptionGood: "Good A", DescriptionBad: "Bad A", SortOrder: 1},
			{ID: uuid.NewString(), Name: "Metric B", DescriptionGood: "Good B", DescriptionBad: "Bad B", SortOrder: 2},
		},
		CreatedAt: time.Now(),
	}
	for i := range tmpl.Metrics {
		tmpl.Metrics[i].TemplateID = tmpl.ID
	}

	if err := store.CreateTemplate(tmpl); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, _ := store.FindTemplateByID(tmpl.ID)
	if got == nil || len(got.Metrics) != 2 {
		t.Fatalf("expected template with 2 metrics, got %+v", got)
	}

	// Delete custom template should succeed
	if err := store.DeleteTemplate(tmpl.ID); err != nil {
		t.Fatalf("delete custom template: %v", err)
	}

	// Delete nonexistent should error
	if err := store.DeleteTemplate("nonexistent"); err == nil {
		t.Error("expected error deleting nonexistent template")
	}
}

func TestFindAllHealthChecks_WithFilters(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{ID: uuid.NewString(), Name: "Filter Team", Members: []string{}, CreatedAt: now, UpdatedAt: now}
	store.CreateTeam(team)

	// Create 3 health checks: 2 open, 1 closed
	for i, status := range []domain.Status{domain.StatusOpen, domain.StatusOpen, domain.StatusClosed} {
		hc := &domain.HealthCheck{
			ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
			Name: fmt.Sprintf("HC %d", i), Status: status, CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		store.CreateHealthCheck(hc)
	}

	// All health checks
	all, err := store.FindAllHealthChecks(domain.HealthCheckFilter{Limit: 10})
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	// Filter by team
	byTeam, _ := store.FindAllHealthChecks(domain.HealthCheckFilter{TeamID: &team.ID, Limit: 10})
	if len(byTeam) != 3 {
		t.Errorf("expected 3 for team, got %d", len(byTeam))
	}

	// Filter by status
	openStatus := domain.StatusOpen
	byStatus, _ := store.FindAllHealthChecks(domain.HealthCheckFilter{Status: &openStatus, Limit: 10})
	if len(byStatus) != 2 {
		t.Errorf("expected 2 open, got %d", len(byStatus))
	}

	// Filter by both
	closedStatus := domain.StatusClosed
	both, _ := store.FindAllHealthChecks(domain.HealthCheckFilter{TeamID: &team.ID, Status: &closedStatus, Limit: 10})
	if len(both) != 1 {
		t.Errorf("expected 1 closed for team, got %d", len(both))
	}

	// Default limit
	defaultLimit, _ := store.FindAllHealthChecks(domain.HealthCheckFilter{})
	if len(defaultLimit) != 3 {
		t.Errorf("expected 3 with default limit, got %d", len(defaultLimit))
	}
}

func TestDeleteHealthCheck(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{ID: uuid.NewString(), Name: "Del Team", Members: []string{}, CreatedAt: now, UpdatedAt: now}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "To Delete", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	// Delete should succeed
	if err := store.DeleteHealthCheck(hc.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Verify gone
	got, _ := store.FindHealthCheckByID(hc.ID)
	if got != nil {
		t.Error("expected nil after delete")
	}

	// Delete nonexistent should error
	if err := store.DeleteHealthCheck("nonexistent"); err == nil {
		t.Error("expected error deleting nonexistent")
	}
}

func TestRemoveTeamMember_NotFound(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	team := &domain.Team{ID: uuid.NewString(), Name: "Team X", Members: []string{"Alice"}, CreatedAt: now, UpdatedAt: now}
	store.CreateTeam(team)

	err := store.RemoveTeamMember(team.ID, "Nobody")
	if err == nil {
		t.Error("expected error removing nonexistent member")
	}
}

func TestDeleteTeam_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.DeleteTeam("nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent team")
	}
}

func TestFindTeamByID_NotFound(t *testing.T) {
	store := newTestStore(t)
	got, err := store.FindTeamByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent team")
	}
}

func TestFindHealthCheckByID_NotFound(t *testing.T) {
	store := newTestStore(t)
	got, err := store.FindHealthCheckByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent health check")
	}
}
