package config

// Config is the root configuration struct for the MCP Zero-Trust Proxy.
type Config struct {
	Server           ServerConfig           `yaml:"server"`
	Auth             AuthConfig             `yaml:"auth"`
	UserRoles        UserRolesConfig        `yaml:"user_roles"`
	UserRestrictions UserRestrictionsConfig `yaml:"user_restrictions"`
	Roles            []RoleConfig           `yaml:"roles"`
	RateLimit        RateLimitConfig        `yaml:"rate_limit"`
	Audit            AuditConfig            `yaml:"audit"`
	Logging          LogConfig              `yaml:"logging"`
	CORS             CORSConfig             `yaml:"cors"`
	License          LicenseConfig          `yaml:"license"`
}

// LicenseConfig holds the optional license key for enabling paid tiers.
// When Key is empty, the proxy runs in free tier.
// Supports ${ENV_VAR} syntax for secret injection from environment variables.
type LicenseConfig struct {
	// Key is the JWT license key string. Empty = free tier.
	// Supports ${ENV_VAR} syntax (e.g., "${LICENSE_KEY}").
	Key string `yaml:"key"`
}

// ClaimRule defines a rule for mapping OAuth claims to RBAC roles.
type ClaimRule struct {
	// Claim is the OAuth claim name to evaluate (e.g., "groups", "department").
	Claim string `yaml:"claim"`
	// Operator specifies how to compare the claim value.
	// Accepted values: "equals", "contains", "starts_with", "ends_with", "regex".
	Operator string `yaml:"operator"`
	// Value is the value to compare against the claim.
	Value string `yaml:"value"`
	// Role is the RBAC role to assign if this rule matches.
	Role string `yaml:"role"`
}

// UserRolesConfig maps authenticated user emails to RBAC role names and supports claim-based role assignment.
type UserRolesConfig struct {
	// Mapping maps email addresses to role names. Role names must be valid built-in roles.
	Mapping map[string]string `yaml:"mapping"`
	// Default is the role assigned to authenticated users not in Mapping. Default: "readonly".
	Default string `yaml:"default"`
	// ClaimMapping is a list of rules to match OAuth claims to roles.
	// Rules are evaluated in order; the first matching rule determines the role.
	// If no claim rule matches, falls back to email mapping or default.
	ClaimMapping []ClaimRule `yaml:"claim_mapping"`
}

// UserRestrictionsConfig controls access based on email regex patterns.
type UserRestrictionsConfig struct {
	// AllowRegex, if set, restricts access to users whose email matches this regex.
	// All other authenticated users receive a 403 error.
	// Default: "" (disabled — all authenticated users are allowed)
	AllowRegex string `yaml:"allow_regex"`
	// DenyRegex, if set, denies access to users whose email matches this regex.
	// Deny takes precedence over allow. Default: "" (disabled)
	DenyRegex string `yaml:"deny_regex"`
}

// CORSConfig controls Cross-Origin Resource Sharing headers.
type CORSConfig struct {
	// AllowedOrigins lists origins permitted to make cross-origin requests.
	// If empty, no CORS headers are added.
	AllowedOrigins []string `yaml:"allowed_origins"`
	// AllowedMethods lists HTTP methods permitted in CORS requests.
	AllowedMethods []string `yaml:"allowed_methods"`
	// AllowedHeaders lists request headers permitted in CORS requests.
	AllowedHeaders []string `yaml:"allowed_headers"`
	// MaxAge is the Access-Control-Max-Age value in seconds.
	MaxAge int `yaml:"max_age"`
}

// ServerConfig holds the proxy's network configuration.
type ServerConfig struct {
	// UpstreamURL is the URL of the upstream MCP server to proxy to. Required.
	UpstreamURL string `yaml:"upstream_url"`
	// ListenAddr is the address the proxy listens on. Default: ":8080"
	ListenAddr string `yaml:"listen_addr"`
	// MaxBodySize is the maximum request body size in bytes. Default: 1048576 (1MB).
	// Requests exceeding this limit are rejected with 413 before any parsing.
	MaxBodySize int64 `yaml:"max_body_size"`
	// TLS holds optional TLS termination configuration.
	TLS TLSConfig `yaml:"tls"`
	// SSE holds configurable SSE proxy settings.
	SSE SSEConfig `yaml:"sse"`
}

// TLSConfig holds TLS termination settings. When CertFile and KeyFile are both
// non-empty, the server starts with TLS enabled.
type TLSConfig struct {
	// CertFile is the path to the TLS certificate file (PEM format).
	CertFile string `yaml:"cert_file"`
	// KeyFile is the path to the TLS private key file (PEM format).
	KeyFile string `yaml:"key_file"`
}

// SSEConfig controls Server-Sent Events proxy behavior.
type SSEConfig struct {
	// TimeoutSeconds is the SSE connection timeout in seconds.
	// 0 means no timeout (connections live until client disconnects). Default: 0.
	TimeoutSeconds int `yaml:"timeout_seconds"`
	// MaxBufferBytes is the maximum scanner buffer size for SSE lines in bytes.
	// Lines exceeding this trigger backpressure (connection closed). Default: 65536 (64KB).
	MaxBufferBytes int `yaml:"max_buffer_bytes"`
}

// AuthConfig holds OAuth 2.1 PKCE authentication provider configuration.
type AuthConfig struct {
	// Provider is the OAuth provider to use. Accepted values: "github", "google", "oidc".
	Provider string `yaml:"provider"`
	// ClientID is the OAuth application client ID.
	ClientID string `yaml:"client_id"`
	// ClientSecret is the OAuth application client secret.
	// Supports ${ENV_VAR} syntax for secret injection from environment variables.
	ClientSecret string `yaml:"client_secret"`
	// IssuerURL is the OIDC issuer URL (required when provider = "oidc").
	IssuerURL string `yaml:"issuer_url"`
	// RedirectURL is the OAuth callback URL registered with the provider.
	RedirectURL string `yaml:"redirect_url"`
}

// RoleConfig defines an RBAC role and its tool-level permissions.
type RoleConfig struct {
	// Name is the role identifier. Built-in roles: "admin", "readonly", "restricted".
	Name string `yaml:"name"`
	// AllowedTools lists the MCP tool names this role may call.
	// Empty slice means all tools are allowed (applies to "admin" role).
	AllowedTools []string `yaml:"allowed_tools"`
	// DenyTools lists MCP tool names explicitly denied for this role.
	// Deny rules take precedence over allow rules.
	DenyTools []string `yaml:"deny_tools"`
}

// RateLimitConfig controls per-client request rate limiting via a token bucket.
type RateLimitConfig struct {
	// RequestsPerMinute is the sustained request rate allowed per client. Default: 100.
	RequestsPerMinute int `yaml:"requests_per_minute"`
	// BurstSize is the maximum burst of requests allowed above the sustained rate. Default: 10.
	BurstSize int `yaml:"burst_size"`
}

// AuditConfig controls the immutable audit trail output.
type AuditConfig struct {
	// Enabled controls whether audit logging is active. Default: true.
	Enabled bool `yaml:"enabled"`
	// Output is the audit log destination. Accepted values: "stdout", "file", "both". Default: "stdout".
	Output string `yaml:"output"`
	// FilePath is the path to the audit log file (required when output = "file" or "both").
	FilePath string `yaml:"file_path"`
	// Rotation controls log file rotation settings.
	Rotation AuditRotationConfig `yaml:"rotation"`
}

// AuditRotationConfig controls audit log file rotation.
type AuditRotationConfig struct {
	// MaxSizeMB is the maximum size of a single audit log file in megabytes before rotation.
	MaxSizeMB int `yaml:"max_size_mb"`
	// MaxAgeHours is the maximum age of audit log files in hours before deletion.
	MaxAgeHours int `yaml:"max_age_hours"`
}

// LogConfig controls the proxy's operational log output.
type LogConfig struct {
	// Level sets the minimum log level. Accepted values: "debug", "info", "warn", "error". Default: "info".
	Level string `yaml:"level"`
	// Format controls log output format. Accepted values: "json", "text". Default: "json".
	Format string `yaml:"format"`
}
