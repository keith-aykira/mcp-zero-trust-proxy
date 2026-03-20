package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: write a temp YAML file and return its path
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

// TestLoad_ValidYAML verifies a fully specified valid config loads correctly.
func TestLoad_ValidYAML(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
  listen_addr: ":9090"
auth:
  provider: "github"
  client_id: "abc123"
  client_secret: "secret"
  redirect_url: "http://localhost:9090/callback"
roles:
  - name: "admin"
    allowed_tools: []
    deny_tools: []
  - name: "readonly"
    allowed_tools: ["tools/list", "resources/read"]
    deny_tools: []
rate_limit:
  requests_per_minute: 200
  burst_size: 20
audit:
  enabled: true
  output: "stdout"
logging:
  level: "debug"
  format: "json"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading valid config: %v", err)
	}

	if cfg.Server.UpstreamURL != "http://localhost:3000" {
		t.Errorf("expected upstream_url %q, got %q", "http://localhost:3000", cfg.Server.UpstreamURL)
	}
	if cfg.Server.ListenAddr != ":9090" {
		t.Errorf("expected listen_addr %q, got %q", ":9090", cfg.Server.ListenAddr)
	}
	if cfg.Auth.Provider != "github" {
		t.Errorf("expected auth provider %q, got %q", "github", cfg.Auth.Provider)
	}
	if len(cfg.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(cfg.Roles))
	}
	if cfg.RateLimit.RequestsPerMinute != 200 {
		t.Errorf("expected 200 req/min, got %d", cfg.RateLimit.RequestsPerMinute)
	}
	if cfg.RateLimit.BurstSize != 20 {
		t.Errorf("expected burst size 20, got %d", cfg.RateLimit.BurstSize)
	}
}

// TestLoad_MissingUpstreamURL verifies that a missing server.upstream_url returns a descriptive error.
func TestLoad_MissingUpstreamURL(t *testing.T) {
	yaml := `
server:
  listen_addr: ":8080"
auth:
  provider: "github"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load should not return error (validation is separate): %v", err)
	}

	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing upstream_url, got nil")
	}
	if !strings.Contains(err.Error(), "upstream_url") {
		t.Errorf("error should mention 'upstream_url', got: %v", err)
	}
}

// TestLoad_InvalidYAMLSyntax verifies that malformed YAML returns a parse error.
func TestLoad_InvalidYAMLSyntax(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
  listen_addr: [this is invalid yaml: {
`
	path := writeTempYAML(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected parse error for invalid YAML syntax, got nil")
	}
}

// TestLoad_Defaults verifies default values are applied when optional fields are omitted.
func TestLoad_Defaults(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
auth:
  provider: "github"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.ListenAddr != ":8080" {
		t.Errorf("expected default listen_addr %q, got %q", ":8080", cfg.Server.ListenAddr)
	}
	if cfg.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("expected default requests_per_minute 100, got %d", cfg.RateLimit.RequestsPerMinute)
	}
	if cfg.RateLimit.BurstSize != 10 {
		t.Errorf("expected default burst_size 10, got %d", cfg.RateLimit.BurstSize)
	}
	if cfg.Audit.Output != "stdout" {
		t.Errorf("expected default audit output %q, got %q", "stdout", cfg.Audit.Output)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level %q, got %q", "info", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected default log format %q, got %q", "json", cfg.Logging.Format)
	}
}

// TestLoad_DefaultAuditEnabled verifies that audit.enabled defaults to true.
func TestLoad_DefaultAuditEnabled(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Audit.Enabled {
		t.Error("expected audit.enabled to default to true")
	}
}

// TestValidate_ValidRoleNames verifies that known role names pass validation.
func TestValidate_ValidRoleNames(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
auth:
  provider: "github"
roles:
  - name: "admin"
  - name: "readonly"
  - name: "restricted"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid role names to pass validation, got: %v", err)
	}
}

// TestValidate_UnknownRoleName verifies that unknown role names return a descriptive error.
func TestValidate_UnknownRoleName(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
auth:
  provider: "github"
roles:
  - name: "superuser"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unknown role name, got nil")
	}
	if !strings.Contains(err.Error(), "superuser") {
		t.Errorf("error should mention the invalid role name 'superuser', got: %v", err)
	}
}

// TestLoad_EmptyRolesDefaultsToBuiltIn verifies that an empty roles array gets the 3 built-in roles.
func TestLoad_EmptyRolesDefaultsToBuiltIn(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
auth:
  provider: "github"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Roles) != 3 {
		t.Errorf("expected 3 default roles, got %d", len(cfg.Roles))
	}

	roleNames := make(map[string]bool)
	for _, r := range cfg.Roles {
		roleNames[r.Name] = true
	}
	for _, expected := range []string{"admin", "readonly", "restricted"} {
		if !roleNames[expected] {
			t.Errorf("expected built-in role %q to be present in defaults", expected)
		}
	}
}

// TestLoad_EnvVarSubstitution verifies that ${ENV_VAR} syntax is resolved from the environment.
func TestLoad_EnvVarSubstitution(t *testing.T) {
	t.Setenv("TEST_MCP_SECRET", "my-secret-value")

	yaml := `
server:
  upstream_url: "http://localhost:3000"
auth:
  provider: "github"
  client_secret: "${TEST_MCP_SECRET}"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Auth.ClientSecret != "my-secret-value" {
		t.Errorf("expected env var substitution to produce %q, got %q", "my-secret-value", cfg.Auth.ClientSecret)
	}
}

// TestLoad_FileNotFound verifies that loading a non-existent file returns an error.
func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error when loading non-existent file, got nil")
	}
}
