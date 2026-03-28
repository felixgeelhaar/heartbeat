package domain

import (
	"fmt"
	"time"
)

// VoteColor is a value object representing the traffic-light vote choice.
type VoteColor string

const (
	VoteGreen  VoteColor = "green"
	VoteYellow VoteColor = "yellow"
	VoteRed    VoteColor = "red"
)

// ParseVoteColor validates and returns a VoteColor from a string.
func ParseVoteColor(s string) (VoteColor, error) {
	switch VoteColor(s) {
	case VoteGreen, VoteYellow, VoteRed:
		return VoteColor(s), nil
	default:
		return "", fmt.Errorf("invalid vote color %q: must be green, yellow, or red", s)
	}
}

// Score returns the numeric value of a vote: green=3, yellow=2, red=1.
func (c VoteColor) Score() float64 {
	switch c {
	case VoteGreen:
		return 3
	case VoteYellow:
		return 2
	case VoteRed:
		return 1
	default:
		return 0
	}
}

// Vote represents a single participant's assessment of one metric in a health check session.
type Vote struct {
	ID            string
	HealthCheckID string
	MetricName    string
	Participant   string
	Color         VoteColor
	Comment       string
	CreatedAt     time.Time
}

// VoteRepository defines persistence operations for votes.
type VoteRepository interface {
	UpsertVote(vote *Vote) error
	FindVotesByHealthCheck(healthCheckID string) ([]*Vote, error)
}
