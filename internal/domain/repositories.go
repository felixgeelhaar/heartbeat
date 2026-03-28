package domain

// Repositories bundles all repository interfaces for dependency injection.
// Since Go does not allow embedding interfaces with same-named methods of
// different signatures, each repository uses prefixed method names
// (e.g., CreateTeam, CreateHealthCheck) to enable composition.
type Repositories interface {
	TeamRepository
	HealthCheckRepository
	VoteRepository
	TemplateRepository
	ActionRepository
}
