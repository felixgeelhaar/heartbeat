package lifecycle_test

import (
	"os"
	"testing"
	"time"

	bolt "github.com/felixgeelhaar/bolt"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/domain"
	"github.com/felixgeelhaar/go-teamhealthcheck/internal/lifecycle"
)

func newLifecycle(t *testing.T) *lifecycle.StatekitLifecycle {
	t.Helper()
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	lc, err := lifecycle.New(logger)
	if err != nil {
		t.Fatalf("create lifecycle: %v", err)
	}
	return lc
}

func openHC() *domain.HealthCheck {
	return &domain.HealthCheck{
		ID:        "hc-test",
		TeamID:    "team-1",
		Status:    domain.StatusOpen,
		CreatedAt: time.Now(),
	}
}

func closedHC() *domain.HealthCheck {
	now := time.Now()
	return &domain.HealthCheck{
		ID:       "hc-closed",
		TeamID:   "team-1",
		Status:   domain.StatusClosed,
		ClosedAt: &now,
	}
}

func TestTransition_OpenToClose_WithVotes(t *testing.T) {
	lc := newLifecycle(t)
	hc := openHC()

	err := lc.Transition(hc, domain.EventClose, 3)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hc.Status != domain.StatusClosed {
		t.Errorf("expected status %q, got %q", domain.StatusClosed, hc.Status)
	}
	if hc.ClosedAt == nil {
		t.Error("expected ClosedAt to be set after closing")
	}
}

func TestTransition_OpenToClose_WithoutVotes(t *testing.T) {
	lc := newLifecycle(t)
	hc := openHC()

	err := lc.Transition(hc, domain.EventClose, 0)
	if err == nil {
		t.Fatal("expected error when closing without votes")
	}
	// Status must not change when guard fails
	if hc.Status != domain.StatusOpen {
		t.Errorf("expected status to remain %q, got %q", domain.StatusOpen, hc.Status)
	}
}

func TestTransition_ClosedToArchive(t *testing.T) {
	lc := newLifecycle(t)
	hc := closedHC()

	err := lc.Transition(hc, domain.EventArchive, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hc.Status != domain.StatusArchived {
		t.Errorf("expected status %q, got %q", domain.StatusArchived, hc.Status)
	}
}

func TestTransition_ClosedToReopen(t *testing.T) {
	lc := newLifecycle(t)
	hc := closedHC()
	original := *hc.ClosedAt

	err := lc.Transition(hc, domain.EventReopen, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if hc.Status != domain.StatusOpen {
		t.Errorf("expected status %q, got %q", domain.StatusOpen, hc.Status)
	}
	if hc.ClosedAt != nil {
		t.Errorf("expected ClosedAt to be nil after reopen, had %v", original)
	}
}

func TestTransition_ArchivedToAny_Fails(t *testing.T) {
	lc := newLifecycle(t)

	events := []domain.LifecycleEvent{
		domain.EventClose,
		domain.EventReopen,
		domain.EventArchive,
	}

	for _, event := range events {
		t.Run(string(event), func(t *testing.T) {
			hc := &domain.HealthCheck{
				ID:     "hc-archived",
				Status: domain.StatusArchived,
			}
			err := lc.Transition(hc, event, 10)
			if err == nil {
				t.Errorf("expected error transitioning archived health check with event %q", event)
			}
		})
	}
}

func TestTransition_UnknownStatus(t *testing.T) {
	lc := newLifecycle(t)

	hc := &domain.HealthCheck{
		ID:     "hc-unknown",
		Status: domain.Status("bogus"),
	}

	err := lc.Transition(hc, domain.EventClose, 5)
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
}

func TestTransition_VoteCountExactlyOne(t *testing.T) {
	lc := newLifecycle(t)
	hc := openHC()

	err := lc.Transition(hc, domain.EventClose, 1)
	if err != nil {
		t.Fatalf("voteCount=1 should satisfy guard, got: %v", err)
	}
	if hc.Status != domain.StatusClosed {
		t.Errorf("expected closed, got %q", hc.Status)
	}
}

func TestNew_ReturnsValidLifecycle(t *testing.T) {
	logger := bolt.New(bolt.NewConsoleHandler(os.Stderr))
	lc, err := lifecycle.New(logger)
	if err != nil {
		t.Fatalf("expected no error creating lifecycle, got: %v", err)
	}
	if lc == nil {
		t.Fatal("expected non-nil lifecycle")
	}
}

func TestTransition_FullCycle_OpenCloseReopen(t *testing.T) {
	lc := newLifecycle(t)
	hc := openHC()

	// Close
	if err := lc.Transition(hc, domain.EventClose, 5); err != nil {
		t.Fatalf("close: %v", err)
	}
	if hc.Status != domain.StatusClosed {
		t.Fatalf("expected closed, got %q", hc.Status)
	}

	// Reopen
	if err := lc.Transition(hc, domain.EventReopen, 0); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if hc.Status != domain.StatusOpen {
		t.Fatalf("expected open after reopen, got %q", hc.Status)
	}
	if hc.ClosedAt != nil {
		t.Error("ClosedAt should be nil after reopen")
	}
}

func TestTransition_FullCycle_OpenCloseArchive(t *testing.T) {
	lc := newLifecycle(t)
	hc := openHC()

	if err := lc.Transition(hc, domain.EventClose, 2); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := lc.Transition(hc, domain.EventArchive, 0); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if hc.Status != domain.StatusArchived {
		t.Errorf("expected archived, got %q", hc.Status)
	}

	// Any transition from archived must fail
	err := lc.Transition(hc, domain.EventReopen, 0)
	if err == nil {
		t.Error("expected error reopening archived health check")
	}
}
