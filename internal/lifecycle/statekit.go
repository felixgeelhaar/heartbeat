// Package lifecycle implements the HealthCheckLifecycle interface using statekit.
package lifecycle

import (
	"fmt"
	"time"

	bolt "go.klarlabs.de/bolt"
	"go.klarlabs.de/statekit"

	"github.com/felixgeelhaar/heartbeat/internal/domain"
)

type healthCheckContext struct {
	HealthCheck *domain.HealthCheck
	VoteCount   int
}

// StatekitLifecycle implements domain.HealthCheckLifecycle using statekit state machines.
type StatekitLifecycle struct {
	openMachine   *statekit.MachineConfig[healthCheckContext]
	closedMachine *statekit.MachineConfig[healthCheckContext]
	logger        *bolt.Logger
}

// New builds the statekit-based health check lifecycle manager.
func New(logger *bolt.Logger) (*StatekitLifecycle, error) {
	openMachine, err := statekit.NewMachine[healthCheckContext]("healthcheck-open").
		WithInitial("open").
		WithGuard("hasVotes", func(ctx healthCheckContext, e statekit.Event) bool {
			return ctx.VoteCount > 0
		}).
		WithAction("setClosedAt", func(ctx *healthCheckContext, e statekit.Event) {
			now := time.Now()
			ctx.HealthCheck.Status = domain.StatusClosed
			ctx.HealthCheck.ClosedAt = &now
		}).
		State("open").
		On("CLOSE").Target("closed").Guard("hasVotes").Do("setClosedAt").Done().
		State("closed").Final().Done().
		Build()
	if err != nil {
		return nil, fmt.Errorf("build open machine: %w", err)
	}

	closedMachine, err := statekit.NewMachine[healthCheckContext]("healthcheck-closed").
		WithInitial("closed").
		WithAction("setArchived", func(ctx *healthCheckContext, e statekit.Event) {
			ctx.HealthCheck.Status = domain.StatusArchived
		}).
		WithAction("clearClosedAt", func(ctx *healthCheckContext, e statekit.Event) {
			ctx.HealthCheck.Status = domain.StatusOpen
			ctx.HealthCheck.ClosedAt = nil
		}).
		State("closed").
		On("ARCHIVE").Target("archived").Do("setArchived").
		On("REOPEN").Target("reopened").Do("clearClosedAt").Done().
		State("archived").Final().Done().
		State("reopened").Final().Done().
		Build()
	if err != nil {
		return nil, fmt.Errorf("build closed machine: %w", err)
	}

	return &StatekitLifecycle{
		openMachine:   openMachine,
		closedMachine: closedMachine,
		logger:        logger,
	}, nil
}

// Transition implements domain.HealthCheckLifecycle.
func (sm *StatekitLifecycle) Transition(hc *domain.HealthCheck, event domain.LifecycleEvent, voteCount int) error {
	var machine *statekit.MachineConfig[healthCheckContext]
	switch hc.Status {
	case domain.StatusOpen:
		machine = sm.openMachine
	case domain.StatusClosed:
		machine = sm.closedMachine
	case domain.StatusArchived:
		return fmt.Errorf("health check %q is archived, no transitions allowed", hc.ID)
	default:
		return fmt.Errorf("unknown status %q for health check %q", hc.Status, hc.ID)
	}

	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *healthCheckContext) {
		ctx.HealthCheck = hc
		ctx.VoteCount = voteCount
	})
	interp.Start()

	prevStatus := hc.Status
	interp.Send(statekit.Event{Type: statekit.EventType(event)})

	if hc.Status == prevStatus {
		return fmt.Errorf("transition %q not allowed from state %q (guard condition not met)", event, prevStatus)
	}

	sm.logger.Info().
		Str("healthcheck_id", hc.ID).
		Str("event", string(event)).
		Str("from", string(prevStatus)).
		Str("to", string(hc.Status)).
		Msg("health check state transition")

	return nil
}
