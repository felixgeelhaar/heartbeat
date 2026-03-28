package domain

import "time"

// Team is an aggregate root representing a group of people who run health checks together.
type Team struct {
	ID        string
	Name      string
	Members   []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TeamRepository defines persistence operations for the Team aggregate.
type TeamRepository interface {
	CreateTeam(team *Team) error
	FindTeamByID(id string) (*Team, error)
	FindAllTeams() ([]*Team, error)
	DeleteTeam(id string) error
	AddTeamMember(teamID, name string) error
	RemoveTeamMember(teamID, name string) error
}
