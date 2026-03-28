package storage_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/domain"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/storage"
	"github.com/felixgeelhaar/go-teamhealthcheck/sdk"
)

func newSDKAdapter(t *testing.T) (*storage.Store, *storage.SDKStoreReader) {
	t.Helper()
	store := newTestStore(t)
	adapter := storage.NewSDKStoreReader(store)
	return store, adapter
}

func TestSDKAdapter_NewSDKStoreReader(t *testing.T) {
	_, adapter := newSDKAdapter(t)
	if adapter == nil {
		t.Fatal("expected non-nil SDKStoreReader")
	}
}

func TestSDKAdapter_FindTeamByID_NotFound(t *testing.T) {
	_, adapter := newSDKAdapter(t)

	got, err := adapter.FindTeamByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent team")
	}
}

func TestSDKAdapter_FindTeamByID_Found(t *testing.T) {
	store, adapter := newSDKAdapter(t)
	now := time.Now()

	team := &domain.Team{
		ID:        uuid.NewString(),
		Name:      "SDK Test Team",
		Members:   []string{"Alice", "Bob"},
		CreatedAt: now,
		UpdatedAt: now,
	}
	store.CreateTeam(team)

	got, err := adapter.FindTeamByID(team.ID)
	if err != nil {
		t.Fatalf("find team: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil team")
	}
	if got.ID != team.ID {
		t.Errorf("expected ID %q, got %q", team.ID, got.ID)
	}
	if got.Name != "SDK Test Team" {
		t.Errorf("expected name 'SDK Test Team', got %q", got.Name)
	}
	if len(got.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(got.Members))
	}
}

func TestSDKAdapter_FindAllTeams(t *testing.T) {
	store, adapter := newSDKAdapter(t)
	now := time.Now()

	// Initially empty
	teams, err := adapter.FindAllTeams()
	if err != nil {
		t.Fatalf("find all teams: %v", err)
	}
	if len(teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(teams))
	}

	// Create two teams
	for _, name := range []string{"SDK Team A", "SDK Team B"} {
		store.CreateTeam(&domain.Team{
			ID: uuid.NewString(), Name: name,
			Members: []string{}, CreatedAt: now, UpdatedAt: now,
		})
	}

	teams, err = adapter.FindAllTeams()
	if err != nil {
		t.Fatalf("find all teams after create: %v", err)
	}
	if len(teams) != 2 {
		t.Errorf("expected 2 teams, got %d", len(teams))
	}
}

func TestSDKAdapter_FindHealthCheckByID_NotFound(t *testing.T) {
	_, adapter := newSDKAdapter(t)

	got, err := adapter.FindHealthCheckByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent HC")
	}
}

func TestSDKAdapter_FindHealthCheckByID_Found(t *testing.T) {
	store, adapter := newSDKAdapter(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "SDK HC Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "SDK Sprint", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	got, err := adapter.FindHealthCheckByID(hc.ID)
	if err != nil {
		t.Fatalf("find HC: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil health check")
	}
	if got.ID != hc.ID {
		t.Errorf("expected ID %q, got %q", hc.ID, got.ID)
	}
	if got.Status != "open" {
		t.Errorf("expected status 'open', got %q", got.Status)
	}
}

func TestSDKAdapter_FindAllHealthChecks(t *testing.T) {
	store, adapter := newSDKAdapter(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "SDK Filter Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	for i, status := range []domain.Status{domain.StatusOpen, domain.StatusClosed} {
		_ = i
		hc := &domain.HealthCheck{
			ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
			Name: "HC", Status: status, CreatedAt: now,
		}
		store.CreateHealthCheck(hc)
	}

	// Find all
	all, err := adapter.FindAllHealthChecks(sdk.HealthCheckFilter{Limit: 10})
	if err != nil {
		t.Fatalf("find all HCs: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 HCs, got %d", len(all))
	}

	// Filter by team
	all2, err := adapter.FindAllHealthChecks(sdk.HealthCheckFilter{TeamID: &team.ID, Limit: 10})
	if err != nil {
		t.Fatalf("find all HCs with team filter: %v", err)
	}
	if len(all2) != 2 {
		t.Errorf("expected 2 HCs for team, got %d", len(all2))
	}

	// Filter by status
	openStatus := "open"
	open, err := adapter.FindAllHealthChecks(sdk.HealthCheckFilter{Status: &openStatus, Limit: 10})
	if err != nil {
		t.Fatalf("find open HCs: %v", err)
	}
	if len(open) != 1 {
		t.Errorf("expected 1 open HC, got %d", len(open))
	}
}

func TestSDKAdapter_FindTemplateByID_NotFound(t *testing.T) {
	_, adapter := newSDKAdapter(t)

	got, err := adapter.FindTemplateByID("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent template")
	}
}

func TestSDKAdapter_FindTemplateByID_Found(t *testing.T) {
	store, adapter := newSDKAdapter(t)

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")

	got, err := adapter.FindTemplateByID(tmpl.ID)
	if err != nil {
		t.Fatalf("find template: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil template")
	}
	if got.Name != "Spotify Squad Health Check" {
		t.Errorf("expected Spotify template, got %q", got.Name)
	}
	if !got.BuiltIn {
		t.Error("expected built_in=true")
	}
	if len(got.Metrics) != 10 {
		t.Errorf("expected 10 metrics, got %d", len(got.Metrics))
	}
}

func TestSDKAdapter_FindVotesByHealthCheck_Empty(t *testing.T) {
	store, adapter := newSDKAdapter(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "SDK Vote Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "SDK Vote HC", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	votes, err := adapter.FindVotesByHealthCheck(hc.ID)
	if err != nil {
		t.Fatalf("find votes: %v", err)
	}
	if len(votes) != 0 {
		t.Errorf("expected 0 votes, got %d", len(votes))
	}
}

func TestSDKAdapter_FindVotesByHealthCheck_WithVotes(t *testing.T) {
	store, adapter := newSDKAdapter(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "SDK Vote Team 2",
		Members: []string{"Alice"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "SDK Vote HC 2", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	store.UpsertVote(&domain.Vote{
		ID: uuid.NewString(), HealthCheckID: hc.ID,
		MetricName: "Fun", Participant: "Alice", Color: domain.VoteGreen, CreatedAt: now,
	})
	store.UpsertVote(&domain.Vote{
		ID: uuid.NewString(), HealthCheckID: hc.ID,
		MetricName: "Speed", Participant: "Alice", Color: domain.VoteYellow, CreatedAt: now,
	})

	votes, err := adapter.FindVotesByHealthCheck(hc.ID)
	if err != nil {
		t.Fatalf("find votes: %v", err)
	}
	if len(votes) != 2 {
		t.Errorf("expected 2 votes, got %d", len(votes))
	}
	// Verify conversion of VoteColor
	for _, v := range votes {
		if v.Color != sdk.VoteGreen && v.Color != sdk.VoteYellow {
			t.Errorf("unexpected color: %q", v.Color)
		}
	}
}

func TestSDKAdapter_ConvertHealthCheck_WithClosedAt(t *testing.T) {
	store, adapter := newSDKAdapter(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Closed HC Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	closedAt := now.Add(time.Hour)
	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Closed Sprint", Status: domain.StatusClosed,
		CreatedAt: now, ClosedAt: &closedAt,
	}
	store.CreateHealthCheck(hc)
	store.UpdateHealthCheck(hc)

	got, err := adapter.FindHealthCheckByID(hc.ID)
	if err != nil {
		t.Fatalf("find HC: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil HC")
	}
	// ClosedAt should be propagated
	if got.ClosedAt == nil {
		t.Error("expected ClosedAt to be propagated through SDK adapter")
	}
}

func TestSDKAdapter_SetEventBus(t *testing.T) {
	// SetEventBus is on Store (not the adapter), but verify it doesn't panic
	store := newTestStore(t)
	// Setting bus twice should not panic
	store.SetEventBus(nil)
}
