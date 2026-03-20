package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
)

// Middleware processes an MCP request through the security pipeline.
// It can modify the request (e.g., strip sensitive fields) or return an error to deny it.
type Middleware interface {
	// Process evaluates the request and identity against the middleware's policy.
	// Returns the (possibly modified) request on success, or an error to deny the request.
	Process(ctx context.Context, req *proxy.MCPRequest, identity *proxy.ClientIdentity) (*proxy.MCPRequest, error)
}

// AuditEntry is an immutable record of a single MCP request passing through the proxy.
type AuditEntry struct {
	Timestamp     time.Time     `json:"timestamp"`
	ClientID      string        `json:"client_id"`
	SessionID     string        `json:"session_id"`
	Method        string        `json:"method"`
	ToolName      string        `json:"tool_name,omitempty"`
	Allowed       bool          `json:"allowed"`
	DeniedReason  string        `json:"denied_reason,omitempty"`
	Latency       time.Duration `json:"latency_ms"`
	RequestID     string        `json:"request_id"`
}

// Authenticator validates an incoming HTTP request and extracts the client identity.
type Authenticator interface {
	// Authenticate validates the request's credentials and returns the client identity.
	// Returns an error if authentication fails.
	Authenticate(r *http.Request) (*proxy.ClientIdentity, error)
}

// RateLimiter enforces per-client request rate limits using a token bucket algorithm.
type RateLimiter interface {
	// Allow returns true if the client is within their rate limit, false if the request should be throttled.
	Allow(clientID string) bool
}

// AuditLogger writes immutable audit entries to the configured output destination.
type AuditLogger interface {
	// Log records an audit entry. Implementations must be non-blocking and safe for concurrent use.
	Log(entry AuditEntry)
}
