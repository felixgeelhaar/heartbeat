package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/config"
)

func TestLoad_MissingFile(t *testing.T) {
	cfg := config.Load("/nonexistent/path/config.yaml")
	if cfg == nil {
		t.Fatal("expected non-nil config for missing file")
	}
	if cfg.Plugins == nil {
		t.Error("expected non-nil Plugins map")
	}
	if cfg.WebhookURL != "" {
		t.Errorf("expected empty WebhookURL, got %q", cfg.WebhookURL)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	content := `
webhook_url: "https://hooks.example.com/test"
plugins:
  reporting:
    enabled: true
  analytics:
    enabled: false
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write config: %v", err)
	}
	f.Close()

	cfg := config.Load(f.Name())
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.WebhookURL != "https://hooks.example.com/test" {
		t.Errorf("expected webhook URL, got %q", cfg.WebhookURL)
	}
	if len(cfg.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(cfg.Plugins))
	}
}

func TestLoad_EmptyYAML(t *testing.T) {
	f, err := os.CreateTemp("", "config-empty-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	cfg := config.Load(f.Name())
	if cfg == nil {
		t.Fatal("expected non-nil config for empty file")
	}
	if cfg.Plugins == nil {
		t.Error("expected non-nil Plugins map after empty file parse")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	f, err := os.CreateTemp("", "config-invalid-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	f.WriteString(":::invalid yaml:::")
	f.Close()

	// Load should not panic on invalid YAML — returns partial or default config.
	cfg := config.Load(f.Name())
	if cfg == nil {
		t.Fatal("expected non-nil config even for invalid YAML")
	}
}

func TestLoad_PluginsNilAfterUnmarshal(t *testing.T) {
	// YAML with webhook but no plugins key at all
	content := `webhook_url: "https://example.com"`
	f, err := os.CreateTemp("", "config-noplugins-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	cfg := config.Load(f.Name())
	if cfg.Plugins == nil {
		t.Error("expected Plugins map to be initialized even when not in YAML")
	}
}

func TestIsPluginEnabled_KnownEnabled(t *testing.T) {
	content := `
plugins:
  reporting:
    enabled: true
  analytics:
    enabled: false
`
	f, err := os.CreateTemp("", "config-plugin-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	cfg := config.Load(f.Name())

	if !cfg.IsPluginEnabled("reporting") {
		t.Error("expected reporting plugin to be enabled")
	}
	if cfg.IsPluginEnabled("analytics") {
		t.Error("expected analytics plugin to be disabled")
	}
}

func TestIsPluginEnabled_UnknownPlugin(t *testing.T) {
	cfg := config.Load("/nonexistent/path/config.yaml")
	if cfg.IsPluginEnabled("nonexistent") {
		t.Error("expected false for unknown plugin")
	}
}

func TestIsPluginEnabled_EmptyConfig(t *testing.T) {
	content := `plugins: {}`
	f, err := os.CreateTemp("", "config-empty-plugins-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	cfg := config.Load(f.Name())
	if cfg.IsPluginEnabled("anything") {
		t.Error("expected false when plugins map is empty")
	}
}

func TestLoad_AuthConfig(t *testing.T) {
	content := `
auth:
  mode: oidc
  oidc:
    issuer: https://accounts.google.com
    client_id: my-client-id
    scopes:
      - openid
      - profile
      - email
`
	f, err := os.CreateTemp("", "config-auth-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	cfg := config.Load(f.Name())
	if cfg.Auth.Mode != "oidc" {
		t.Errorf("expected auth mode oidc, got %q", cfg.Auth.Mode)
	}
	if cfg.Auth.OIDC.Issuer != "https://accounts.google.com" {
		t.Errorf("expected issuer, got %q", cfg.Auth.OIDC.Issuer)
	}
	if cfg.Auth.OIDC.ClientID != "my-client-id" {
		t.Errorf("expected client_id, got %q", cfg.Auth.OIDC.ClientID)
	}
	if len(cfg.Auth.OIDC.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(cfg.Auth.OIDC.Scopes))
	}
}

func TestLoad_DefaultAuthMode(t *testing.T) {
	cfg := config.Load("/nonexistent/path")
	if cfg.Auth.Mode != "" {
		t.Errorf("expected empty auth mode for default config, got %q", cfg.Auth.Mode)
	}
}

func TestLoad_ReturnsDefaultForDirectory(t *testing.T) {
	dir := t.TempDir()
	// Pass a path that is actually a directory — ReadFile will fail.
	cfg := config.Load(filepath.Join(dir))
	if cfg == nil {
		t.Fatal("expected default config")
	}
}

func TestLoad_MultiplePlugins(t *testing.T) {
	content := `
plugins:
  alpha:
    enabled: true
  beta:
    enabled: true
  gamma:
    enabled: false
`
	f, err := os.CreateTemp("", "config-multi-*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	cfg := config.Load(f.Name())

	cases := []struct {
		name    string
		enabled bool
	}{
		{"alpha", true},
		{"beta", true},
		{"gamma", false},
		{"delta", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cfg.IsPluginEnabled(tc.name)
			if got != tc.enabled {
				t.Errorf("IsPluginEnabled(%q) = %v, want %v", tc.name, got, tc.enabled)
			}
		})
	}
}
