package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/rs/zerolog/log"
)

const defaultToolsCacheTTL = 60 * time.Second // 60 seconds cache TTL

// serverRouter implements path-based routing to multiple upstream servers.
type serverRouter struct {
	mu          sync.RWMutex
	servers     map[string]*url.URL  // name -> upstream URL
	serverTimeouts map[string]int    // name -> timeout in seconds
	defaultServer string            // optional fallback
	toolsCache    map[string]*toolsCacheEntry // server name -> cached tools
	cacheTTL      time.Duration     // cache TTL
	httpClient    *http.Client      // client for cache refresh
}

// toolsCacheEntry holds cached tools list with expiry time.
type toolsCacheEntry struct {
	tools []config.ToolInfo
	expiresAt time.Time
}

// NewServerRouter creates a router from ServerRegistryConfig.
func NewServerRouter(cfg *config.ServerRegistryConfig) (*serverRouter, error) {
	r := &serverRouter{
	servers:        make(map[string]*url.URL),
	serverTimeouts: make(map[string]int),
	toolsCache: map[string]*toolsCacheEntry{},
	cacheTTL: defaultToolsCacheTTL,
httpClient: &http.Client{
			Timeout: 10 * time.Second, // Short timeout for cache refresh
		},
	}

	if cfg != nil && strings.TrimSpace(cfg.Default) != "" {
		r.defaultServer = cfg.Default
	}

	for _, srv := range cfg.Servers {
	if !srv.Enabled {
			continue
		}
		parsed, err := url.Parse(srv.URL)
		if err != nil {
			return nil, fmt.Errorf("parse server URL %q: %w", srv.URL, err)
		}
		r.servers[srv.Name] = parsed
		r.serverTimeouts[srv.Name] = srv.Timeout
	}

	return r, nil
}

// ResolveForRequest resolves a request path to an upstream server URL.
func (r *serverRouter) ResolveForRequest(req *http.Request) (*url.URL, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path := req.URL.Path

	// Strip leading slash
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// Extract server name from first path segment
	parts := strings.SplitN(path, "/", 2)
	serverName := parts[0]

	// Look up server
	if upstream, ok := r.servers[serverName]; ok {
		return upstream, serverName, nil
	}

	// Fallback to default server
	if r.defaultServer != "" {
		if upstream, ok := r.servers[r.defaultServer]; ok {
			return upstream, r.defaultServer, nil
		}
	}

	return nil, "", ErrUnknownServer
}

// GetServerUpstream returns the upstream URL for a specific server by name.
func (r *serverRouter) GetServerUpstream(serverName string) (*url.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	upstream, ok := r.servers[serverName]
	if !ok {
		return nil, &RouterError{Code: 404, Message: fmt.Sprintf("server %q not found", serverName)}
	}
	return upstream, nil
}

// GetServerTimeout returns the HTTP timeout for a specific server.
func (r *serverRouter) GetServerTimeout(serverName string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if timeout, ok := r.serverTimeouts[serverName]; ok {
		return timeout
	}
	return 120 // default 120 seconds
}

// GetServerNames returns list of enabled server names.
func (r *serverRouter) GetServerNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	return names
}

// RefreshToolsCache fetches and caches tools list for all servers.
func (r *serverRouter) RefreshToolsCache() error {
	r.mu.RLock()
	serversCopy := make(map[string]*url.URL)
	for k, v := range r.servers {
		serversCopy[k] = v
	}
	r.mu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(serversCopy))

	for serverName, upstreamURL := range serversCopy {
		wg.Add(1)
		go func(name string, u *url.URL) {
			defer wg.Done()
			tools, err := fetchToolsList(u)
			if err != nil {
				errChan <- fmt.Errorf("fetch tools for %q: %w", name, err)
				return
			}
			r.updateToolsCache(name, tools)
		}(serverName, upstreamURL)
	}

	wg.Wait()
	close(errChan)

	// Collect errors but don't fail completely (lazy refresh will retry)
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// fetchToolsList fetches tools list from a single MCP server.
func fetchToolsList(upstreamURL *url.URL) ([]config.ToolInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build tools/list JSON-RPC request
	reqBody := []byte(`{"jsonrpc":"2.0","method":"tools/list","params":{},"id":1}`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL.String(), strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a short-lived client for this request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	// Parse response to extract tools
	var rpcResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("upstream error: %s", rpcResp.Error.Message)
	}

	// Parse tools list from result
	var result struct {
		Tools []config.ToolInfo `json:"tools"`
	}

	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

// updateToolsCache updates the tools cache for a specific server.
func (r *serverRouter) updateToolsCache(serverName string, tools []config.ToolInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.toolsCache[serverName] = &toolsCacheEntry{
		tools:     tools,
		expiresAt: time.Now().Add(r.cacheTTL),
	}
}

// GetAggregatedTools returns merged tools list from all enabled servers.
// Each tool is prefixed with server-name. (e.g., "files.read_file")
// On collision, tools are renamed to "{name}.{server}".
func (r *serverRouter) GetAggregatedTools() []config.ToolInfo {
	// Collect all tools with their server names
	type toolSource struct {
		tool  config.ToolInfo
		server string
	}

	var allTools []toolSource

	for serverName := range r.toolsCache {
		tools, isFresh := r.getToolsFromCache(serverName)
		if !isFresh {
			// Lazy refresh on cache miss (non-blocking)
			go r.lazyRefreshTools(serverName)
			continue
		}
		for _, tool := range tools {
			allTools = append(allTools, toolSource{tool: tool, server: serverName})
		}
	}

	// Build aggregated list with namespace prefix
	aggregated := make([]config.ToolInfo, 0, len(allTools))
	globalNames := map[string]bool{} // Track global tool names for collision detection

	for _, ts := range allTools {
		tool := ts.tool
		namespacedName := fmt.Sprintf("%s.%s", ts.server, tool.Name)

		// Check for collision at global scope
		if globalNames[tool.Name] {
			// Collision detected — rename with server suffix and log
			collisionName := fmt.Sprintf("%s.%s", tool.Name, ts.server)
			log.Info().
				Str("original_name", tool.Name).
				Str("renamed_to", collisionName).
				Str("server", ts.server).
				Msg("Tool name collision — renaming with server suffix")

			newTool := tool
			newTool.Name = collisionName
			aggregated = append(aggregated, newTool)
		} else {
			// No collision — use namespaced version
			globalNames[tool.Name] = true
			newTool := tool
			newTool.Name = namespacedName
			aggregated = append(aggregated, newTool)
		}
	}

	return aggregated
}

// getToolsFromCache returns tools from cache if not expired.
func (r *serverRouter) getToolsFromCache(serverName string) ([]config.ToolInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.toolsCache[serverName]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	// Return copy to avoid race
	tools := make([]config.ToolInfo, len(entry.tools))
	copy(tools, entry.tools)
	return tools, true
}

// lazyRefreshTools triggers a non-blocking cache refresh for a server.
func (r *serverRouter) lazyRefreshTools(serverName string) {
	upstream, err := r.GetServerUpstream(serverName)
	if err != nil {
		log.Warn().Err(err).Str("server", serverName).Msg("Failed to get server upstream for lazy refresh")
		return
	}

	tools, err := fetchToolsList(upstream)
	if err != nil {
		log.Warn().Err(err).Str("server", serverName).Msg("Failed to lazy refresh tools cache")
		return
	}

	r.updateToolsCache(serverName, tools)
	log.Debug().Str("server", serverName).Int("tools", len(tools)).Msg("Lazy refreshed tools cache")
}
