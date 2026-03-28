package storage_test

import (
	"os"
	"testing"
	"time"

	bolt "github.com/felixgeelhaar/bolt"
	"github.com/google/uuid"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/domain"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/storage"
)

func TestStoreDB_NonNil(t *testing.T) {
	store := newTestStore(t)
	if store.DB() == nil {
		t.Error("expected DB() to return non-nil *sql.DB")
	}
}

func TestAllBuiltInTemplatesSeeded(t *testing.T) {
	store := newTestStore(t)

	expectedNames := []string{
		"Spotify Squad Health Check",
		"Team Maturity (Tuckman)",
		"Psychological Safety (Edmondson)",
		"DORA Metrics (DevOps)",
	}

	for _, name := range expectedNames {
		tmpl, err := store.FindTemplateByName(name)
		if err != nil {
			t.Errorf("find template %q: %v", name, err)
			continue
		}
		if tmpl == nil {
			t.Errorf("template %q not seeded", name)
			continue
		}
		if !tmpl.BuiltIn {
			t.Errorf("template %q should be built_in=true", name)
		}
		if len(tmpl.Metrics) == 0 {
			t.Errorf("template %q has no metrics", name)
		}
	}
}

func TestActionCRUD(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	// Set up team and health check
	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Action Team",
		Members: []string{"Alice"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Action HC", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	// CreateAction
	action := &domain.Action{
		ID:            uuid.NewString(),
		HealthCheckID: hc.ID,
		MetricName:    "Fun",
		Description:   "Improve team fun activities",
		Assignee:      "Alice",
		Completed:     false,
		CreatedAt:     now,
	}
	if err := store.CreateAction(action); err != nil {
		t.Fatalf("create action: %v", err)
	}

	// FindActionsByHealthCheck
	actions, err := store.FindActionsByHealthCheck(hc.ID)
	if err != nil {
		t.Fatalf("find actions: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Description != "Improve team fun activities" {
		t.Errorf("unexpected description: %q", actions[0].Description)
	}
	if actions[0].Completed {
		t.Error("expected action to be incomplete initially")
	}
	if actions[0].CompletedAt != nil {
		t.Error("expected CompletedAt to be nil initially")
	}

	// CompleteAction
	if err := store.CompleteAction(action.ID); err != nil {
		t.Fatalf("complete action: %v", err)
	}

	// Verify completed
	actions2, _ := store.FindActionsByHealthCheck(hc.ID)
	if len(actions2) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions2))
	}
	if !actions2[0].Completed {
		t.Error("expected action to be completed")
	}
	if actions2[0].CompletedAt == nil {
		t.Error("expected CompletedAt to be set after completion")
	}
}

func TestCompleteAction_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.CompleteAction("nonexistent-action-id")
	if err == nil {
		t.Error("expected error completing nonexistent action")
	}
}

func TestFindActionsByHealthCheck_Empty(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Empty Action Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Empty HC", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	actions, err := store.FindActionsByHealthCheck(hc.ID)
	if err != nil {
		t.Fatalf("find actions: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestActionCRUD_MultipleActions(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Multi Action Team",
		Members: []string{"Alice", "Bob"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Multi Action HC", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	// Create multiple actions
	for i, desc := range []string{"Action 1", "Action 2", "Action 3"} {
		_ = i
		action := &domain.Action{
			ID:            uuid.NewString(),
			HealthCheckID: hc.ID,
			MetricName:    "Fun",
			Description:   desc,
			Assignee:      "Alice",
			CreatedAt:     now,
		}
		if err := store.CreateAction(action); err != nil {
			t.Fatalf("create action %q: %v", desc, err)
		}
	}

	actions, err := store.FindActionsByHealthCheck(hc.ID)
	if err != nil {
		t.Fatalf("find actions: %v", err)
	}
	if len(actions) != 3 {
		t.Errorf("expected 3 actions, got %d", len(actions))
	}
}

func TestCustomTemplateCRUD_FindByName(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmplID := uuid.NewString()
	tmpl := &domain.Template{
		ID:          tmplID,
		Name:        "My Custom Template",
		Description: "A custom template for testing",
		BuiltIn:     false,
		Metrics: []domain.TemplateMetric{
			{ID: uuid.NewString(), TemplateID: tmplID, Name: "Metric X", DescriptionGood: "Good X", DescriptionBad: "Bad X", SortOrder: 1},
		},
		CreatedAt: now,
	}

	if err := store.CreateTemplate(tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	// Find by name
	found, err := store.FindTemplateByName("My Custom Template")
	if err != nil {
		t.Fatalf("find by name: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find custom template by name")
	}
	if found.ID != tmplID {
		t.Errorf("expected ID %q, got %q", tmplID, found.ID)
	}
	if len(found.Metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(found.Metrics))
	}
}

func TestFindTemplateByName_NotFound(t *testing.T) {
	store := newTestStore(t)

	found, err := store.FindTemplateByName("Nonexistent Template")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Error("expected nil for nonexistent template name")
	}
}

func TestCreateTemplate_MultipleMetrics(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmplID := uuid.NewString()
	tmpl := &domain.Template{
		ID:          tmplID,
		Name:        "Multi Metric Template",
		Description: "Template with many metrics",
		BuiltIn:     false,
		Metrics: []domain.TemplateMetric{
			{ID: uuid.NewString(), TemplateID: tmplID, Name: "M1", DescriptionGood: "Good 1", DescriptionBad: "Bad 1", SortOrder: 1},
			{ID: uuid.NewString(), TemplateID: tmplID, Name: "M2", DescriptionGood: "Good 2", DescriptionBad: "Bad 2", SortOrder: 2},
			{ID: uuid.NewString(), TemplateID: tmplID, Name: "M3", DescriptionGood: "Good 3", DescriptionBad: "Bad 3", SortOrder: 3},
		},
		CreatedAt: now,
	}

	if err := store.CreateTemplate(tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}

	found, err := store.FindTemplateByID(tmplID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find template")
	}
	if len(found.Metrics) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(found.Metrics))
	}
}

func TestStoreClose(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	store, err := storage.New(":memory:", logger)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
}

func TestEventBus_PublishOnVote(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Event Team",
		Members: []string{"Alice"}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Event HC", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	// Upsert vote (no bus set — must not panic)
	vote := &domain.Vote{
		ID:            uuid.NewString(),
		HealthCheckID: hc.ID,
		MetricName:    "Fun",
		Participant:   "Alice",
		Color:         domain.VoteGreen,
		CreatedAt:     now,
	}
	if err := store.UpsertVote(vote); err != nil {
		t.Fatalf("upsert vote without bus: %v", err)
	}
}

func TestFindAllHealthChecks_DefaultLimit(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Limit Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	for i := 0; i < 5; i++ {
		hc := &domain.HealthCheck{
			ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
			Name: "HC", Status: domain.StatusOpen,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		store.CreateHealthCheck(hc)
	}

	// Zero limit should use default
	all, err := store.FindAllHealthChecks(domain.HealthCheckFilter{Limit: 0})
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 health checks with zero limit, got %d", len(all))
	}
}

func TestUpdateHealthCheck_StatusAndClosedAt(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()

	tmpl, _ := store.FindTemplateByName("Spotify Squad Health Check")
	team := &domain.Team{
		ID: uuid.NewString(), Name: "Update Team",
		Members: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	store.CreateTeam(team)

	hc := &domain.HealthCheck{
		ID: uuid.NewString(), TeamID: team.ID, TemplateID: tmpl.ID,
		Name: "Update HC", Status: domain.StatusOpen, CreatedAt: now,
	}
	store.CreateHealthCheck(hc)

	// Update to closed with ClosedAt
	closedAt := now.Add(time.Hour)
	hc.Status = domain.StatusClosed
	hc.ClosedAt = &closedAt

	if err := store.UpdateHealthCheck(hc); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := store.FindHealthCheckByID(hc.ID)
	if got.Status != domain.StatusClosed {
		t.Errorf("expected closed, got %q", got.Status)
	}
	if got.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}

	// Update to archived
	hc.Status = domain.StatusArchived
	if err := store.UpdateHealthCheck(hc); err != nil {
		t.Fatalf("update to archived: %v", err)
	}

	got2, _ := store.FindHealthCheckByID(hc.ID)
	if got2.Status != domain.StatusArchived {
		t.Errorf("expected archived, got %q", got2.Status)
	}
}

func TestDeleteBuiltInTemplates_AllBlocked(t *testing.T) {
	store := newTestStore(t)

	names := []string{
		"Spotify Squad Health Check",
		"Team Maturity (Tuckman)",
		"Psychological Safety (Edmondson)",
		"DORA Metrics (DevOps)",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			tmpl, err := store.FindTemplateByName(name)
			if err != nil || tmpl == nil {
				t.Fatalf("template %q not found", name)
			}
			err = store.DeleteTemplate(tmpl.ID)
			if err == nil {
				t.Errorf("expected error when deleting built-in template %q", name)
			}
		})
	}
}
