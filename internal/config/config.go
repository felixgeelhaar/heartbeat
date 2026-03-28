package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Mode string   `yaml:"mode"` // "token" (default), "oidc"
	OIDC OIDCAuth `yaml:"oidc"`
}

// OIDCAuth holds OIDC provider settings.
type OIDCAuth struct {
	Issuer   string   `yaml:"issuer"`
	ClientID string   `yaml:"client_id"`
	Scopes   []string `yaml:"scopes"`
}

// Config holds the application configuration.
type Config struct {
	Auth       AuthConfig              `yaml:"auth"`
	Plugins    map[string]PluginConfig `yaml:"plugins"`
	WebhookURL string                  `yaml:"webhook_url"`
}

// PluginConfig holds per-plugin configuration.
type PluginConfig struct {
	Enabled bool `yaml:"enabled"`
}

// Load reads configuration from a YAML file.
// Returns a default config if the file doesn't exist.
func Load(path string) *Config {
	cfg := &Config{
		Plugins: make(map[string]PluginConfig),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	yaml.Unmarshal(data, cfg)
	if cfg.Plugins == nil {
		cfg.Plugins = make(map[string]PluginConfig)
	}
	return cfg
}

// IsPluginEnabled returns true if the named plugin is enabled in config.
// Plugins not mentioned in config are disabled by default.
func (c *Config) IsPluginEnabled(name string) bool {
	pc, ok := c.Plugins[name]
	if !ok {
		return false
	}
	return pc.Enabled
}
