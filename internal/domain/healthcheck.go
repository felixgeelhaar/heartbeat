package domain

import "time"

// Status is a value object for the lifecycle state of a health check session.
type Status string

const (
	StatusOpen     Status = "open"
	StatusClosed   Status = "closed"
	StatusArchived Status = "archived"
)

// HealthCheck is the aggregate root for a single health check session.
type HealthCheck struct {
	ID         string
	TeamID     string
	TemplateID string
	Name       string
	Anonymous  bool
	Status     Status
	CreatedAt  time.Time
	ClosedAt   *time.Time
}

// IsOpen returns true if the health check is still accepting votes.
func (h *HealthCheck) IsOpen() bool {
	return h.Status == StatusOpen
}

// IsVotable returns true if the health check accepts votes (only when open).
func (h *HealthCheck) IsVotable() bool {
	return h.Status == StatusOpen
}

// HealthCheckRepository defines persistence operations for the HealthCheck aggregate.
type HealthCheckRepository interface {
	CreateHealthCheck(hc *HealthCheck) error
	FindHealthCheckByID(id string) (*HealthCheck, error)
	FindAllHealthChecks(filter HealthCheckFilter) ([]*HealthCheck, error)
	UpdateHealthCheck(hc *HealthCheck) error
	DeleteHealthCheck(id string) error
}

// HealthCheckFilter specifies optional criteria for listing health checks.
type HealthCheckFilter struct {
	TeamID *string
	Status *Status
	Limit  int
}
