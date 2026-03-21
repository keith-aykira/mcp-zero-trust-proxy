package proxy

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
)

// contextKey is an unexported type for context keys defined in this package.
// Using a custom type prevents collisions with keys from other packages.
type contextKey string

const (
	// MCPRequestKey is the context key for storing the parsed *MCPRequest.
	// Middleware and handlers downstream can retrieve it via:
	//   req.Context().Value(MCPRequestKey).(*MCPRequest)
	MCPRequestKey contextKey = "mcp_request"

	// ClientIdentityKey is the context key for storing the *ClientIdentity after auth.
	ClientIdentityKey contextKey = "client_identity"
)

// Handler is the core reverse proxy. It parses incoming JSON-RPC 2.0 requests,
// stores the parsed data in context for middleware, and forwards requests to
// the upstream MCP server. For SSE connections, it delegates to ProxySSE.
//
// Middleware fields (authenticator, rateLimiter, auditLogger) are left as
// interface{} placeholders here — concrete types will be wired in Plans 02-03
// through 02-05 once the dependency structure is resolved. Using interface{} here
// avoids an import cycle between proxy (the HTTP layer) and middleware (which
// imports proxy types for MCPRequest/ClientIdentity).
type Handler struct {
	upstream     *url.URL
	reverseProxy *httputil.ReverseProxy
	httpClient   *http.Client

	// sseTimeout is the configurable SSE connection timeout. 0 = no timeout.
	sseTimeout time.Duration
	// sseMaxBuffer is the configurable SSE scanner buffer size in bytes.
	sseMaxBuffer int
}

// NewHandler constructs a Handler from config. Returns an error if the upstream URL
// is missing or cannot be parsed.
func NewHandler(cfg *config.Config) (*Handler, error) {
	if cfg.Server.UpstreamURL == "" {
		return nil, fmt.Errorf("upstream_url is required")
	}

	upstream, err := url.Parse(cfg.Server.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("parse upstream_url %q: %w", cfg.Server.UpstreamURL, err)
	}
	if upstream.Scheme == "" || upstream.Host == "" {
		return nil, fmt.Errorf("upstream_url %q must have scheme and host", cfg.Server.UpstreamURL)
	}

	// HTTP client with sensible timeouts.
	// SSE connections use a separate client with no read timeout (streaming).
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}

	// httputil.ReverseProxy handles standard HTTP forwarding.
	rp := httputil.NewSingleHostReverseProxy(upstream)
	// Preserve original request Host header.
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Host = upstream.Host
		// Strip the client's Authorization header so upstream operators cannot
		// steal client OAuth tokens. The proxy has already validated the token;
		// upstream must not receive it.
		req.Header.Del("Authorization")
	}

	// SSE configuration: timeout and buffer size from config.
	sseTimeout := time.Duration(cfg.Server.SSE.TimeoutSeconds) * time.Second
	sseMaxBuffer := cfg.Server.SSE.MaxBufferBytes
	if sseMaxBuffer <= 0 {
		sseMaxBuffer = 64 * 1024 // 64KB default
	}

	return &Handler{
		upstream:     upstream,
		reverseProxy: rp,
		httpClient:   httpClient,
		sseTimeout:   sseTimeout,
		sseMaxBuffer: sseMaxBuffer,
	}, nil
}

// SetTransport replaces the ReverseProxy's transport. Primarily for testing —
// allows injection of a custom RoundTripper to intercept or capture requests.
func (h *Handler) SetTransport(t http.RoundTripper) {
	h.reverseProxy.Transport = t
}

// ServeHTTP is the main entry point. It:
//  1. Detects SSE requests (Accept: text/event-stream) and delegates to ProxySSE.
//  2. Forwards all other requests to upstream via httputil.ReverseProxy.
//
// Body reading and JSON-RPC parsing are handled exclusively by Pipeline (HARD-11).
// Handler is a simple pass-through — it does not parse the body or set context values.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route SSE requests to the streaming proxy.
	if isSSERequest(r) {
		upstreamURL := h.upstreamURLForRequest(r)
		if err := ProxySSE(w, r, upstreamURL, h.sseTimeout, h.sseMaxBuffer); err != nil {
			// SSE errors after headers are sent cannot change the status code.
			// Log silently; client will see connection close.
			_ = err
		}
		return
	}

	// Standard HTTP forwarding via httputil.ReverseProxy.
	// ReverseProxy handles 5xx from upstream by passing them through.
	h.reverseProxy.ServeHTTP(w, r)
}

// upstreamURLForRequest builds the full upstream URL for a given request path.
func (h *Handler) upstreamURLForRequest(r *http.Request) *url.URL {
	u := *h.upstream
	u.Path = r.URL.Path
	u.RawQuery = r.URL.RawQuery
	return &u
}

// isSSERequest returns true if the request signals an SSE connection.
// This matches both explicit Accept: text/event-stream and GET requests to
// paths that conventionally serve SSE (e.g., /sse, /events).
func isSSERequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "text/event-stream" || containsSSEMIME(accept)
}

// containsSSEMIME returns true if the Accept header includes text/event-stream.
func containsSSEMIME(accept string) bool {
	for _, part := range splitCSV(accept) {
		if part == "text/event-stream" {
			return true
		}
	}
	return false
}

// splitCSV splits a comma-separated string and trims whitespace from each part.
func splitCSV(s string) []string {
	parts := make([]string, 0)
	for _, p := range bytes.Split([]byte(s), []byte(",")) {
		trimmed := bytes.TrimSpace(p)
		// Strip quality value (e.g., "text/html;q=0.9" -> "text/html")
		if i := bytes.IndexByte(trimmed, ';'); i >= 0 {
			trimmed = bytes.TrimSpace(trimmed[:i])
		}
		if len(trimmed) > 0 {
			parts = append(parts, string(trimmed))
		}
	}
	return parts
}
