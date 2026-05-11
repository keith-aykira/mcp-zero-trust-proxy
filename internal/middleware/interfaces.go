package middleware

import (
	"context"
	"net/http"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

// Middleware processes an MCP request through the security pipeline.
// It can modify the request (e.g., strip sensitive fields) or return an error to deny it.
type Middleware interface {
	// Process evaluates the request and identity against the middleware's policy.
	// Returns the (possibly modified) request on success, or an error to deny the request.
	Process(ctx context.Context, req *proxy.MCPRequest, identity *proxy.ClientIdentity) (*proxy.MCPRequest, error)
}

// AuditEntry is an alias for proxy.AuditEntry. It is re-exported here for backward
// compatibility with code that imports middleware.AuditEntry.
// The canonical definition lives in the proxy package to avoid import cycles.
type AuditEntry = proxy.AuditEntry

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
// This is an alias for proxy.AuditLogger. The canonical definition lives in the proxy
// package to avoid import cycles between proxy and middleware.
type AuditLogger = proxy.AuditLogger
