package proxy

import (
	"context"
	"net/http"
	"net/url"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
)

// Router resolves incoming requests to upstream server URLs.
type Router interface {
	// ResolveForRequest resolves a request path to an upstream server URL.
	// Returns (nil, ErrUnknownServer) for unknown paths.
	// On success, returns both the upstream URL and the server name.
	ResolveForRequest(r *http.Request) (*url.URL, string, error)

	// GetServerUpstream returns the upstream URL for a specific server by name.
	GetServerUpstream(serverName string) (*url.URL, error)

	// GetServerTimeout returns the HTTP timeout for a specific server.
	GetServerTimeout(serverName string) int

	// GetServerNames returns list of enabled server names.
	GetServerNames() []string

	// RefreshToolsCache fetches and caches tools list for all servers.
	RefreshToolsCache() error

	// GetAggregatedTools returns merged tools list from all enabled servers.
	// Each tool is prefixed with server-name. (e.g., "files.read_file")
	// On collision, tools are renamed to "{name}-{server}".
	GetAggregatedTools() []config.ToolInfo
}

// ErrUnknownServer is returned when a request path does not match any configured server.
var ErrUnknownServer = &RouterError{
	Code:    404,
	Message: "unknown server",
}

// RouterError represents router errors with HTTP status codes.
type RouterError struct {
	Code    int
	Message string
}

func (e *RouterError) Error() string {
	return e.Message
}

// IsRouterError checks if an error is a RouterError.
func IsRouterError(err error) bool {
	_, ok := err.(*RouterError)
	return ok
}

// RequestContextKey is the context key for storing routing info.
type RequestContextKey string

const (
	// ResolvedServerKey is the context key for the resolved server name.
	ResolvedServerKey RequestContextKey = "resolved_server"

	// OriginalMethodKey is the context key for the original request method.
	OriginalMethodKey RequestContextKey = "original_method"
)

// WithResolvedServer returns a context with the resolved server name.
func WithResolvedServer(ctx context.Context, serverName string) context.Context {
	return context.WithValue(ctx, ResolvedServerKey, serverName)
}

// GetResolvedServer returns the resolved server name from context, or empty string.
func GetResolvedServer(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ResolvedServerKey).(string); ok {
		return v
	}
	return ""
}
