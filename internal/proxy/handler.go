package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/catalog"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
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

// Handler is the core reverse proxy. It routes incoming requests to the appropriate
// upstream MCP server based on path prefix, or aggregates responses for tools/list.
// For SSE connections, it delegates to ProxySSE.
//
// Middleware fields (authenticator, rateLimiter, auditLogger) are left as
// interface{} placeholders here — concrete types will be wired in Plans 02-03
// through 02-05 once the dependency structure is resolved. Using interface{} here
// avoids an import cycle between proxy (the HTTP layer) and middleware (which
// imports proxy types for MCPRequest/ClientIdentity).
type Handler struct {
	router       Router
	httpClient   *http.Client
	toolCatalog  *catalog.ToolCatalog

	// sseTimeout is the configurable SSE connection timeout. 0 = no timeout.
	sseTimeout time.Duration
	// sseMaxBuffer is the configurable SSE scanner buffer size in bytes.
	sseMaxBuffer int
}

// NewHandler constructs a Handler from config. Returns an error if the router is nil.
func NewHandler(cfg *config.Config, router Router, toolCatalog *catalog.ToolCatalog) (*Handler, error) {
	if router == nil {
		return nil, fmt.Errorf("router is required")
	}

	// HTTP client with sensible timeouts.
	// SSE connections use a separate client with no read timeout (streaming).
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}

	// SSE configuration: timeout and buffer size from config.
	sseTimeout := time.Duration(cfg.Server.SSE.TimeoutSeconds) * time.Second
	sseMaxBuffer := cfg.Server.SSE.MaxBufferBytes
	if sseMaxBuffer <= 0 {
		sseMaxBuffer = 64 * 1024 // 64KB default
	}

	return &Handler{
		router:       router,
		httpClient:   httpClient,
		toolCatalog:  toolCatalog,
		sseTimeout:   sseTimeout,
		sseMaxBuffer: sseMaxBuffer,
	}, nil
}

// SetTransport replaces the HTTP client's transport. Primarily for testing —
// allows injection of a custom RoundTripper to intercept or capture requests.
func (h *Handler) SetTransport(t http.RoundTripper) {
	if h.httpClient != nil {
		h.httpClient.Transport = t
	}
}

// ServeHTTP is the main entry point. It:
//  1. Resolves the target upstream server based on path prefix.
//  2. Detects SSE requests (Accept: text/event-stream) and delegates to ProxySSE.
//  3. Forwards all other requests to upstream via httputil.ReverseProxy.
//
// For multi-server mode, paths must start with `/server_name/...`. Requests to
// unknown server names return 404 Not Found.
//
// Body reading and JSON-RPC parsing are handled exclusively by Pipeline (HARD-11).
// Handler is a simple pass-through — it does not parse the body or set context values.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Resolve the upstream server for this request.
	upstream, serverName, err := h.router.ResolveForRequest(r)
	if err != nil {
		if err == ErrUnknownServer {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Route SSE requests to the streaming proxy with the resolved upstream.
	if isSSERequest(r) {
		if err := ProxySSE(w, r, upstream, h.sseTimeout, h.sseMaxBuffer); err != nil {
			// SSE errors after headers are sent cannot change the status code.
			// Log silently; client will see connection close.
			_ = err
		}
		return
	}

	// Set context values for middleware downstream.
	ctx := r.Context()
	ctx = context.WithValue(ctx, ResolvedServerKey, serverName)
	r = r.WithContext(ctx)

	// Create a reverse proxy for this specific upstream.
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Host = upstream.Host
		req.Header.Del("Authorization")
	}
	// Use the HTTP client's transport so SetTransport works for testing.
	if h.httpClient != nil {
		proxy.Transport = h.httpClient.Transport
	}

	// Standard HTTP forwarding via httputil.ReverseProxy.
	// ReverseProxy handles 5xx from upstream by passing them through.
	proxy.ServeHTTP(w, r)
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
