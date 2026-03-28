package storage

import "github.com/felixgeelhaar/go-teamhealthcheck/internal/domain"

// Compile-time verification that *Store implements all domain repository interfaces.
var (
	_ domain.TeamRepository        = (*Store)(nil)
	_ domain.HealthCheckRepository = (*Store)(nil)
	_ domain.VoteRepository        = (*Store)(nil)
	_ domain.TemplateRepository    = (*Store)(nil)
	_ domain.ActionRepository      = (*Store)(nil)
	_ domain.Repositories          = (*Store)(nil)
)
