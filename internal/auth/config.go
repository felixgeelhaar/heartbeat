package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"go.klarlabs.de/mcp/middleware"
)

// UserInfo maps an auth token to a user's identity.
type UserInfo struct {
	Name   string `json:"name"`
	TeamID string `json:"team_id"`
}

// Config holds the authentication configuration.
type Config struct {
	Tokens map[string]UserInfo `json:"tokens"`
}

// LoadConfig reads an auth config file and returns a token validator function
// suitable for use with middleware.BearerTokenAuthenticator.
func LoadConfig(path string) (func(string) *middleware.Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read auth config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse auth config: %w", err)
	}

	tokens := make(map[string]*middleware.Identity, len(cfg.Tokens))
	for token, info := range cfg.Tokens {
		tokens[token] = &middleware.Identity{
			ID:   info.Name,
			Name: info.Name,
			Metadata: map[string]any{
				"team_id": info.TeamID,
			},
		}
	}

	return middleware.StaticTokens(tokens), nil
}
