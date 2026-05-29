package config

// Config is the root configuration struct for the MCP Zero-Trust Proxy.
type Config struct {
	Server           ServerConfig           `yaml:"server"`
	Auth             AuthConfig             `yaml:"auth"`
	UserRoles        UserRolesConfig        `yaml:"user_roles"`
	UserRestrictions UserRestrictionsConfig `yaml:"user_restrictions"`
	Roles            []RoleConfig           `yaml:"roles"`
	Classification   ClassificationConfig   `yaml:"classification"`
	RateLimit        RateLimitConfig        `yaml:"rate_limit"`
	Audit            AuditConfig            `yaml:"audit"`
	Logging          LogConfig              `yaml:"logging"`
	CORS             CORSConfig             `yaml:"cors"`
	License          LicenseConfig          `yaml:"license"`
	PIIMasking       PIIMaskingConfig       `yaml:"pii_masking"`
	Outbound         OutboundConfig         `yaml:"outbound"`
	Catalog          CatalogConfig          `yaml:"catalog"`
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

// OutboundConfig holds TLS settings for outbound HTTP connections (OAuth, upstream MCP, audit sinks).
type OutboundConfig struct {
	// MinTLSVersion sets the minimum TLS version for outbound connections. Valid values: "1.0", "1.1", "1.2", "1.3". Default: "1.2".
	MinTLSVersion string `yaml:"min_tls_version"`
}

// ServerConfig holds the proxy's network configuration.
type ServerConfig struct {
	// UpstreamURL is the URL of the upstream MCP server to proxy to. Required.
	// Deprecated: Use Registry.Servers for multi-server routing. Kept for backward compatibility.
	UpstreamURL string `yaml:"upstream_url"`
	// ListenAddr is the address the proxy listens on. Default: ":8080"
	ListenAddr string `yaml:"listen_addr"`
	// MaxBodySize is the maximum request body size in bytes. Default: 1048576 (1MB).
	// Requests exceeding this limit are rejected with 413 before any parsing.
	MaxBodySize int64 `yaml:"max_body_size"`
	// MinHTTPVersion sets the minimum HTTP version. Valid values: "1.0", "1.1", "2.0". Default: "1.1".
	MinHTTPVersion string `yaml:"min_http_version"`
	// TLS holds optional TLS termination configuration.
	TLS TLSConfig `yaml:"tls"`
	// SSE holds configurable SSE proxy settings.
	SSE SSEConfig `yaml:"sse"`
	// Registry is the multi-server routing configuration. When set, takes precedence over UpstreamURL.
	Registry ServerRegistryConfig `yaml:"registry"`
}

// ServerRegistryConfig defines upstream MCP server registry and routing.
type ServerRegistryConfig struct {
	// Default is the fallback server name. If not set, unknown paths return 404.
	Default string `yaml:"default"`
	// Servers is the list of upstream MCP servers.
	Servers []UpstreamServerConfig `yaml:"servers"`
}

// UpstreamServerConfig defines a single upstream MCP server.
type UpstreamServerConfig struct {
	// Name is the server identifier (used in path routing: /{name}/...).
	Name string `yaml:"name"`
	// URL is the upstream server URL (e.g., "http://localhost:3000").
	URL string `yaml:"url"`
	// Enabled controls whether this server is active. Default: true.
	Enabled bool `yaml:"enabled"`
	// Tags are optional metadata labels for filtering/organization.
	Tags []string `yaml:"tags"`
	// Timeout is the HTTP client timeout for this server in seconds. Default: 120.
	Timeout int `yaml:"timeout"`
}

// ToolInfo holds metadata about a tool from tools/list response.
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// TLSConfig holds TLS termination settings. When CertFile and KeyFile are both
// non-empty, the server starts with TLS enabled.
type TLSConfig struct {
	// CertFile is the path to the TLS certificate file (PEM format).
	CertFile string `yaml:"cert_file"`
	// KeyFile is the path to the TLS private key file (PEM format).
	KeyFile string `yaml:"key_file"`
	// MinVersion sets the minimum TLS version. Valid values: "1.2", "1.3". Default: "1.2".
	MinVersion string `yaml:"min_version"`
	// CipherSuites is an optional list of cipher suite names to use. If empty, Go's secure defaults are applied.
	CipherSuites []string `yaml:"cipher_suites"`
	// Required, when true, mandates TLS. The proxy will refuse to start if
	// CertFile and KeyFile are not both configured.
	Required bool `yaml:"required"`
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

// SessionStoreConfig controls session storage backend and TTL.
type SessionStoreConfig struct {
	// TTL is the session validity window. Sessions are refreshed on successful access.
	// Default: 24h.
	TTL string `yaml:"ttl"`
	// Backend is the session storage backend type. Accepted values: "memory", "file".
	// "memory" keeps sessions in memory only (lost on restart).
	// "file" persists sessions to disk for cross-restart durability.
	// Default: "memory".
	Backend string `yaml:"backend"`
	// Filepath is the path to the session file when backend = "file".
	// Default: "./sessions.json".
	Filepath string `yaml:"filepath"`
}

// AuthConfig holds OAuth 2.1 PKCE authentication provider configuration.
type AuthConfig struct {
	// Provider is the OAuth provider to use. Accepted values: "github", "google", "entra", "oidc".
	Provider string `yaml:"provider"`
	// ClientID is the OAuth application client ID.
	ClientID string `yaml:"client_id"`
	// ClientSecret is the OAuth application client secret.
	// Supports ${ENV_VAR} syntax for secret injection from environment variables.
	ClientSecret string `yaml:"client_secret"`
	// IssuerURL is the OIDC issuer URL (required when provider = "oidc", optional for "entra").
	// For "entra", default is https://login.microsoftonline.com/{tenant_id}/v2.0
	IssuerURL string `yaml:"issuer_url"`
	// RedirectURL is the OAuth callback URL registered with the provider.
	RedirectURL string `yaml:"redirect_url"`
	// MaxTokenCacheSize is the maximum number of entries in the token cache.
	// When the cache exceeds this size, the least recently used entry is evicted.
	// Default: 1000.
	MaxTokenCacheSize int `yaml:"max_token_cache_size"`
	// Session controls session storage configuration.
	Session SessionStoreConfig `yaml:"session"`
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
	// ClassificationLevel sets the highest classification this role may access.
	// Empty string defaults to the lowest level ("Public").
	// Accepted values are those defined in the classification.levels section.
	ClassificationLevel string `yaml:"classification_level,omitempty"`
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
	// Sinks is a list of external audit sinks (OCSF, CEF, JSON-HTTP).
	// Each sink operates independently and asynchronously.
	Sinks []AuditSinkConfig `yaml:"sinks"`
}

// AuditRotationConfig controls audit log file rotation.
type AuditRotationConfig struct {
	// MaxSizeMB is the maximum size of a single audit log file in megabytes before rotation.
	MaxSizeMB int `yaml:"max_size_mb"`
	// MaxAgeHours is the maximum age of audit log files in hours before deletion.
	MaxAgeHours int `yaml:"max_age_hours"`
	// MaxBackups is the maximum number of rotated log files to retain.
	// Default: 1.
	MaxBackups int `yaml:"max_backups"`
}

// AuditSinkConfig defines an external audit sink configuration.
type AuditSinkConfig struct {
	// Type is the sink type. Accepted values: "ocsf", "cef", "json_http".
	Type string `yaml:"type"`
	// Enabled controls whether this sink is active. Default: true.
	Enabled bool `yaml:"enabled"`
	// Name is an optional label for the sink (defaults to type).
	Name string `yaml:"name"`
	// OCSF holds OCSF-specific configuration (when type = "ocsf").
	OCSF *OCSFSinkConfig `yaml:"ocsf,omitempty"`
	// CEF holds CEF-specific configuration (when type = "cef").
	CEF *CEFSinkConfig `yaml:"cef,omitempty"`
	// JSONHTTP holds JSON-HTTP-specific configuration (when type = "json_http").
	JSONHTTP *JSONHTTPSinkConfig `yaml:"json_http,omitempty"`
	// Filter controls which events are sent to this sink.
	Filter *SinkFilterConfig `yaml:"filter,omitempty"`
}

// OCSFSinkConfig holds configuration for the OCSF sink (Azure Log Analytics / Sentinel).
type OCSFSinkConfig struct {
	// WorkspaceID is the Azure Log Analytics workspace ID.
	WorkspaceID string `yaml:"workspace_id"`
	// APIKey is the workspace primary key (supports ${ENV_VAR} syntax).
	APIKey string `yaml:"api_key"`
	// BatchSize is the maximum number of events to batch before sending. Default: 100.
	BatchSize int `yaml:"batch_size"`
	// FlushInterval is the maximum seconds to wait before flushing batch. Default: 5.
	FlushInterval int `yaml:"flush_interval"`
	// Timeout is the HTTP request timeout in seconds. Default: 30.
	Timeout int `yaml:"timeout"`
	// BufferSize is the maximum events to buffer before dropping. Default: 1000.
	BufferSize int `yaml:"buffer_size"`
}

// CEFSinkConfig holds configuration for the CEF sink (syslog/SIEM).
type CEFSinkConfig struct {
	// Transport is the protocol. Accepted values: "udp", "tcp", "tcp_tls", "https". Default: "udp".
	Transport string `yaml:"transport"`
	// Host is the syslog server hostname or IP.
	Host string `yaml:"host"`
	// Port is the port number (514 for UDP, 6514 for TCP-TLS). Default: 514.
	Port int `yaml:"port"`
	// Facility is the syslog facility. Default: "local0".
	Facility string `yaml:"facility"`
	// BatchSize is for HTTPS transport only. Default: 10.
	BatchSize int `yaml:"batch_size"`
	// FlushInterval is for HTTPS transport only. Default: 1.
	FlushInterval int `yaml:"flush_interval"`
	// Timeout is the connection timeout in seconds. Default: 5.
	Timeout int `yaml:"timeout"`
	// BufferSize is the maximum events to buffer before dropping. Default: 1000.
	BufferSize int `yaml:"buffer_size"`
}

// JSONHTTPSinkConfig holds configuration for the JSON-HTTP sink.
type JSONHTTPSinkConfig struct {
	// Endpoint is the HTTPS URL to send events to.
	Endpoint string `yaml:"endpoint"`
	// Headers are custom HTTP headers to include with each request.
	Headers map[string]string `yaml:"headers"`
	// BatchSize is the maximum number of events to batch before sending. Default: 50.
	BatchSize int `yaml:"batch_size"`
	// FlushInterval is the maximum seconds to wait before flushing batch. Default: 10.
	FlushInterval int `yaml:"flush_interval"`
	// Timeout is the HTTP request timeout in seconds. Default: 15.
	Timeout int `yaml:"timeout"`
	// BasicAuth holds basic authentication credentials.
	BasicAuth *BasicAuthConfig `yaml:"basic_auth,omitempty"`
	// BearerToken is the bearer token for authentication (supports ${ENV_VAR} syntax).
	BearerToken string `yaml:"bearer_token,omitempty"`
	// BufferSize is the maximum events to buffer before dropping. Default: 1000.
	BufferSize int `yaml:"buffer_size"`
}

// BasicAuthConfig holds basic authentication credentials.
type BasicAuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"` // Supports ${ENV_VAR} syntax
}

// SinkFilterConfig controls which events are sent to a sink.
// If a field is empty/zero, no filtering is applied on that field.
type SinkFilterConfig struct {
	// Methods is a list of MCP methods to include (e.g., ["tools/call"]). Empty = all.
	Methods []string `yaml:"methods"`
	// Tools is a list of tool names to include (supports wildcards with *). Empty = all.
	Tools []string `yaml:"tools"`
	// Results is a list of outcomes: "allowed", "denied". Empty = both.
	Results []string `yaml:"results"`
	// ClientIDs is a list of client IDs to include. Empty = all.
	ClientIDs []string `yaml:"client_ids"`
}

// LogConfig controls the proxy's operational log output.
type LogConfig struct {
	// Level sets the minimum log level. Accepted values: "debug", "info", "warn", "error". Default: "info".
	Level string `yaml:"level"`
	// Format controls log output format. Accepted values: "json", "text". Default: "json".
	Format string `yaml:"format"`
}

// PIIMaskingConfig holds PII masking configuration.
type PIIMaskingConfig struct {
	// Enabled controls whether PII masking is globally active. Default: false.
	// PII masking is also controlled by sensitivity class assignments per tool.
	Enabled bool `yaml:"enabled"`

	// Patterns is a list of named PII pattern definitions.
	Patterns []PIIPatternConfig `yaml:"patterns"`

	// SensitivityClasses defines PII sensitivity levels that group patterns.
	// Tools are assigned to classes, and all patterns in that class apply.
	SensitivityClasses []PIISensitivityClassConfig `yaml:"sensitivity_classes"`

	// ToolClassAssignments maps tool names to sensitivity class names.
	// Supports wildcards with * (e.g., "*_export" matches "data_export").
	// If a tool is not assigned to any class, no PII masking is applied to it.
	ToolClassAssignments map[string]string `yaml:"tool_class_assignments"`
}

// PIIPatternConfig defines a named PII pattern and its masking behavior.
type PIIPatternConfig struct {
	// Name is the unique identifier for this pattern (e.g., "email", "ssn").
	Name string `yaml:"name"`

	// Pattern is the regex to match PII data. Required.
	Pattern string `yaml:"pattern"`

	// Mask is the replacement string for matched PII (default: "***REDACTED***").
	// Use ${N} to reference capture groups, e.g., "${1}***" shows first group.
	Mask string `yaml:"mask"`

	// PartialMask configures partial masking (e.g., "j***y@smith.com" for emails).
	// If set, this takes precedence over Mask.
	PartialMask *PartialMaskConfig `yaml:"partial_mask"`

	// Description provides documentation for this pattern.
	Description string `yaml:"description"`
}

// PartialMaskConfig configures partial masking to show some characters.
type PartialMaskConfig struct {
	// ShowFirst shows the first N characters from the start (default: 0).
	ShowFirst int `yaml:"show_first"`

	// ShowLast shows the last N characters from the end (default: 0).
	ShowLast int `yaml:"show_last"`

	// Filler character used for masked portion (default: "*").
	Filler string `yaml:"filler"`
}

// PIISensitivityClassConfig groups PII patterns under a named sensitivity level.
type PIISensitivityClassConfig struct {
	// Name is the unique identifier for this sensitivity class (e.g., "high", "medium", "low").
	Name string `yaml:"name"`

	// PatternNames is a list of pattern names included in this class.
	// All listed patterns must be defined in the Patterns section.
	PatternNames []string `yaml:"pattern_names"`

	// Description provides documentation for this class.
	Description string `yaml:"description"`
}

// ClassificationConfig holds information classification settings.
// When empty or not configured, classification is inactive and has no effect.
type ClassificationConfig struct {
	// Levels defines the ordered classification hierarchy from lowest to highest.
	// Default: ["Public", "Sensitive", "Confidential"].
	Levels []string `yaml:"levels"`

	// ToolAssignments maps tool names to their classification levels.
	// Tools not listed default to the lowest level ("Public").
	ToolAssignments map[string]string `yaml:"tool_assignments"`
}

// CatalogConfig holds tool cataloging and caching settings.
type CatalogConfig struct {
	// Enabled controls whether tool cataloging is active. Default: false.
	Enabled bool `yaml:"enabled"`

	// DatabasePath is the SQLite database file path for storing tool metadata.
	// Default: "./tools_catalog.db".
	DatabasePath string `yaml:"database_path"`

	// RefreshInterval is the time between catalog updates (e.g., "5m", "1h").
	// Default: "1h".
	RefreshInterval string `yaml:"refresh_interval"`

	// CacheToolsList controls whether tools/list responses should be served
	// from the catalog cache instead of forwarding to upstream.
	// When true, provides faster responses and consistent filtering.
	// Default: false.
	CacheToolsList bool `yaml:"cache_tools_list"`
}
