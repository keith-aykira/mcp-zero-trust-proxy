package config

// Config is the root configuration struct for the MCP Zero-Trust Proxy.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Roles     []RoleConfig    `yaml:"roles"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Audit     AuditConfig     `yaml:"audit"`
	Logging   LogConfig       `yaml:"logging"`
}

// ServerConfig holds the proxy's network configuration.
type ServerConfig struct {
	// UpstreamURL is the URL of the upstream MCP server to proxy to. Required.
	UpstreamURL string `yaml:"upstream_url"`
	// ListenAddr is the address the proxy listens on. Default: ":8080"
	ListenAddr string `yaml:"listen_addr"`
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
}

// LogConfig controls the proxy's operational log output.
type LogConfig struct {
	// Level sets the minimum log level. Accepted values: "debug", "info", "warn", "error". Default: "info".
	Level string `yaml:"level"`
	// Format controls log output format. Accepted values: "json", "text". Default: "json".
	Format string `yaml:"format"`
}
