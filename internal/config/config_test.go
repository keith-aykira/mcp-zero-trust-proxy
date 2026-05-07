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
	// Built-in roles (admin, readonly, restricted) are always present
	// User-defined roles override built-in configs but don't add duplicates
	if len(cfg.Roles) != 3 {
		t.Errorf("expected 3 roles (admin, readonly, restricted with built-in restricted), got %d", len(cfg.Roles))
	}
	// Verify user-defined roles are correctly merged
	foundAdmin := false
	foundReadOnly := false
	foundRestricted := false
	for _, role := range cfg.Roles {
		if role.Name == "admin" {
			foundAdmin = true
			if len(role.AllowedTools) != 0 {
				t.Errorf("admin role should have empty allowed_tools, got %v", role.AllowedTools)
			}
		}
		if role.Name == "readonly" {
			foundReadOnly = true
			if len(role.AllowedTools) != 2 || role.AllowedTools[0] != "tools/list" || role.AllowedTools[1] != "resources/read" {
				t.Errorf("readonly role should have allowed_tools ['tools/list', 'resources/read'], got %v", role.AllowedTools)
			}
		}
		if role.Name == "restricted" {
			foundRestricted = true
		}
	}
	if !foundAdmin || !foundReadOnly || !foundRestricted {
		t.Errorf("expected all 3 built-in roles to be present, got admin=%v readonly=%v restricted=%v", foundAdmin, foundReadOnly, foundRestricted)
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

// TestValidate_CustomRoleNameAllowed verifies that custom role names are now allowed.
func TestValidate_CustomRoleNameAllowed(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
auth:
  provider: "github"
roles:
  - name: "devops"
    allowed_tools:
      - "tools/list"
      - "tools/call"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	err = Validate(cfg)
	if err != nil {
		t.Errorf("expected custom role names to be allowed, got: %v", err)
	}

	// Verify custom role is in the final config along with built-in roles
	foundDevOps := false
	builtinCount := 0
	for _, role := range cfg.Roles {
		if role.Name == "devops" {
			foundDevOps = true
		}
		if builtInRoleNames[role.Name] {
			builtinCount++
		}
	}
	if !foundDevOps {
		t.Error("custom role 'devops' not found in merged roles")
	}
	if builtinCount != 3 {
		t.Errorf("expected 3 built-in roles, got %d", builtinCount)
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

// TestLoad_UserRolesMapping verifies that user_roles mapping loads correctly.
func TestLoad_UserRolesMapping(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
user_roles:
  default: "readonly"
  mapping:
    "alice@example.com": "admin"
    "bob@example.com": "restricted"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with user_roles: %v", err)
	}

	if cfg.UserRoles.Default != "readonly" {
		t.Errorf("expected user_roles.default %q, got %q", "readonly", cfg.UserRoles.Default)
	}
	if cfg.UserRoles.Mapping["alice@example.com"] != "admin" {
		t.Errorf("expected alice to have admin role, got %q", cfg.UserRoles.Mapping["alice@example.com"])
	}
	if cfg.UserRoles.Mapping["bob@example.com"] != "restricted" {
		t.Errorf("expected bob to have restricted role, got %q", cfg.UserRoles.Mapping["bob@example.com"])
	}
}

// TestLoad_MaxBodySize verifies that max_body_size parses to int64 bytes.
func TestLoad_MaxBodySize(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
  max_body_size: 2097152
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with max_body_size: %v", err)
	}

	if cfg.Server.MaxBodySize != 2097152 {
		t.Errorf("expected max_body_size 2097152, got %d", cfg.Server.MaxBodySize)
	}
}

// TestLoad_MaxBodySizeDefault verifies default max_body_size is 1MB.
func TestLoad_MaxBodySizeDefault(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.MaxBodySize != 1048576 {
		t.Errorf("expected default max_body_size 1048576 (1MB), got %d", cfg.Server.MaxBodySize)
	}
}

// TestLoad_CORSConfig verifies that cors block parses allowed_origins, methods, headers, max_age.
func TestLoad_CORSConfig(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
cors:
  allowed_origins:
    - "https://example.com"
    - "https://app.example.com"
  allowed_methods:
    - "GET"
    - "POST"
    - "OPTIONS"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
  max_age: 3600
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with cors: %v", err)
	}

	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Errorf("expected 2 allowed_origins, got %d", len(cfg.CORS.AllowedOrigins))
	}
	if cfg.CORS.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("expected first origin %q, got %q", "https://example.com", cfg.CORS.AllowedOrigins[0])
	}
	if len(cfg.CORS.AllowedMethods) != 3 {
		t.Errorf("expected 3 allowed_methods, got %d", len(cfg.CORS.AllowedMethods))
	}
	if len(cfg.CORS.AllowedHeaders) != 2 {
		t.Errorf("expected 2 allowed_headers, got %d", len(cfg.CORS.AllowedHeaders))
	}
	if cfg.CORS.MaxAge != 3600 {
		t.Errorf("expected max_age 3600, got %d", cfg.CORS.MaxAge)
	}
}

// TestLoad_TLSConfig verifies that tls block parses cert_file and key_file.
func TestLoad_TLSConfig(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
  tls:
    cert_file: "/etc/ssl/cert.pem"
    key_file: "/etc/ssl/key.pem"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with tls: %v", err)
	}

	if cfg.Server.TLS.CertFile != "/etc/ssl/cert.pem" {
		t.Errorf("expected cert_file %q, got %q", "/etc/ssl/cert.pem", cfg.Server.TLS.CertFile)
	}
	if cfg.Server.TLS.KeyFile != "/etc/ssl/key.pem" {
		t.Errorf("expected key_file %q, got %q", "/etc/ssl/key.pem", cfg.Server.TLS.KeyFile)
	}
}

// TestLoad_SSEConfig verifies that sse.timeout_seconds and sse.max_buffer_bytes parse correctly.
func TestLoad_SSEConfig(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
  sse:
    timeout_seconds: 120
    max_buffer_bytes: 131072
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with sse: %v", err)
	}

	if cfg.Server.SSE.TimeoutSeconds != 120 {
		t.Errorf("expected sse.timeout_seconds 120, got %d", cfg.Server.SSE.TimeoutSeconds)
	}
	if cfg.Server.SSE.MaxBufferBytes != 131072 {
		t.Errorf("expected sse.max_buffer_bytes 131072, got %d", cfg.Server.SSE.MaxBufferBytes)
	}
}

// TestLoad_AuditRotationConfig verifies that audit.rotation block parses max_size_mb and max_age_hours.
func TestLoad_AuditRotationConfig(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
audit:
  enabled: true
  output: "file"
  file_path: "/var/log/audit.jsonl"
  rotation:
    max_size_mb: 100
    max_age_hours: 720
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with audit.rotation: %v", err)
	}

	if cfg.Audit.Rotation.MaxSizeMB != 100 {
		t.Errorf("expected rotation.max_size_mb 100, got %d", cfg.Audit.Rotation.MaxSizeMB)
	}
	if cfg.Audit.Rotation.MaxAgeHours != 720 {
		t.Errorf("expected rotation.max_age_hours 720, got %d", cfg.Audit.Rotation.MaxAgeHours)
	}
}

// TestLoad_UserRolesDefault verifies that user_roles.default defaults to "readonly".
func TestLoad_UserRolesDefault(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.UserRoles.Default != "readonly" {
		t.Errorf("expected default user_roles.default to be %q, got %q", "readonly", cfg.UserRoles.Default)
	}
}

// TestValidate_UserRolesUnknownRole verifies Validate rejects user_roles with undefined custom role names.
func TestValidate_UserRolesUnknownRole(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
user_roles:
  default: "superuser"
  mapping:
    "alice@example.com": "admin"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for undefined custom role in user_roles.default, got nil")
	}
	if !strings.Contains(err.Error(), "superuser") {
		t.Errorf("error should mention undefined role 'superuser', got: %v", err)
	}
}

// TestValidate_UserRolesMappingUnknownRole verifies Validate rejects mapping with undefined custom role name.
func TestValidate_UserRolesMappingUnknownRole(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
user_roles:
  default: "readonly"
  mapping:
    "alice@example.com": "superadmin"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for undefined custom role in user_roles.mapping, got nil")
	}
	if !strings.Contains(err.Error(), "superadmin") {
		t.Errorf("error should mention undefined role 'superadmin', got: %v", err)
	}
}

// TestValidate_UserRolesCustomRoleDefined verifies that custom roles can be used when defined.
func TestValidate_UserRolesCustomRoleDefined(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
roles:
  - name: "devops"
    allowed_tools:
      - "tools/list"
      - "tools/call"
user_roles:
  default: "devops"
  mapping:
    "alice@example.com": "admin"
    "bob@example.com": "devops"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	err = Validate(cfg)
	if err != nil {
		t.Errorf("expected custom roles to be allowed when defined, got: %v", err)
	}
	if cfg.UserRoles.Default != "devops" {
		t.Errorf("expected default role 'devops', got %q", cfg.UserRoles.Default)
	}
	if cfg.UserRoles.Mapping["bob@example.com"] != "devops" {
		t.Errorf("expected bob to have 'devops' role, got %q", cfg.UserRoles.Mapping["bob@example.com"])
	}
}

// TestLoad_LicenseKey verifies that license.key loads correctly from YAML.
func TestLoad_LicenseKey(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
license:
  key: "some.jwt.token"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with license: %v", err)
	}

	if cfg.License.Key != "some.jwt.token" {
		t.Errorf("expected license.key %q, got %q", "some.jwt.token", cfg.License.Key)
	}
}

// TestLoad_LicenseKeyEmpty verifies that omitting license.key results in empty string (free tier).
func TestLoad_LicenseKeyEmpty(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config without license: %v", err)
	}

	if cfg.License.Key != "" {
		t.Errorf("expected empty license.key for free tier, got %q", cfg.License.Key)
	}
}

// TestLoad_LicenseKeyEnvVar verifies that ${ENV_VAR} syntax works for license.key.
func TestLoad_LicenseKeyEnvVar(t *testing.T) {
	t.Setenv("TEST_LICENSE_KEY", "env.jwt.token")

	yaml := `
server:
  upstream_url: "http://localhost:3000"
license:
  key: "${TEST_LICENSE_KEY}"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with license env var: %v", err)
	}

	if cfg.License.Key != "env.jwt.token" {
		t.Errorf("expected license.key from env var %q, got %q", "env.jwt.token", cfg.License.Key)
	}
}

// TestLoad_UserRestrictions verifies that user_restrictions block loads allow_regex and deny_regex correctly.
func TestLoad_UserRestrictions(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
user_restrictions:
  allow_regex: '@company\\.com$'
  deny_regex: '@contractor\\.com$'
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with user_restrictions: %v", err)
	}

	// YAML preserves the backslash, so we get the raw string
	if cfg.UserRestrictions.AllowRegex != "@company\\\\.com$" {
		t.Errorf("expected allow_regex %q, got %q", "@company\\\\.com$", cfg.UserRestrictions.AllowRegex)
	}
	if cfg.UserRestrictions.DenyRegex != "@contractor\\\\.com$" {
		t.Errorf("expected deny_regex %q, got %q", "@contractor\\\\.com$", cfg.UserRestrictions.DenyRegex)
	}
}

// TestValidate_UserRestrictionsInvalidAllowRegex verifies Validate rejects malformed allow_regex.
func TestValidate_UserRestrictionsInvalidAllowRegex(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
user_restrictions:
  allow_regex: "[invalid(regex"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid allow_regex, got nil")
	}
	if !strings.Contains(err.Error(), "allow_regex") {
		t.Errorf("error should mention 'allow_regex', got: %v", err)
	}
}

// TestValidate_UserRestrictionsInvalidDenyRegex verifies Validate rejects malformed deny_regex.
func TestValidate_UserRestrictionsInvalidDenyRegex(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
user_restrictions:
  deny_regex: "[invalid(regex"
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid deny_regex, got nil")
	}
	if !strings.Contains(err.Error(), "deny_regex") {
		t.Errorf("error should mention 'deny_regex', got: %v", err)
	}
}

// TestLoad_UserRestrictionsEmpty verifies that empty user_restrictions are allowed (disabled).
func TestLoad_UserRestrictionsEmpty(t *testing.T) {
	yaml := `
server:
  upstream_url: "http://localhost:3000"
user_restrictions: {}
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading config with empty user_restrictions: %v", err)
	}

	if cfg.UserRestrictions.AllowRegex != "" {
		t.Errorf("expected empty allow_regex, got %q", cfg.UserRestrictions.AllowRegex)
	}
	if cfg.UserRestrictions.DenyRegex != "" {
		t.Errorf("expected empty deny_regex, got %q", cfg.UserRestrictions.DenyRegex)
	}
}
