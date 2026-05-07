// Package integration contains end-to-end tests that verify the full proxy pipeline
// by starting a real HTTP server backed by a mock MCP server and sending actual HTTP requests.
//
// Each test creates:
//   - A MockServer (simulating an upstream MCP server)
//   - A proxy.Pipeline (the security middleware chain)
//   - An httptest.Server wrapping the pipeline
//
// Requests flow: test client -> proxy pipeline -> mock MCP server
package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/rbac"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/ratelimit"
)

// testToken is the bearer token accepted by testAuthenticator.
const testToken = "test-token-abc123"

// testAuthenticator is a mock Authenticator that accepts a single hardcoded token.
// It avoids real OAuth endpoints while still exercising the pipeline's auth step.
type testAuthenticator struct {
	token    string
	identity *proxy.ClientIdentity
}

func (a *testAuthenticator) Authenticate(r *http.Request) (*proxy.ClientIdentity, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("Authorization header must use Bearer scheme")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != a.token {
		return nil, fmt.Errorf("invalid token")
	}
	return a.identity, nil
}

// testAuditLogger captures audit entries in memory for assertions.
// Uses a mutex to be safe for concurrent use (HTTP servers call Log from multiple goroutines).
type testAuditLogger struct {
	mu      sync.Mutex
	entries []proxy.AuditEntry
}

func (l *testAuditLogger) Log(entry proxy.AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

// snapshot returns a copy of entries captured so far, safe to read after all requests complete.
func (l *testAuditLogger) snapshot() []proxy.AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]proxy.AuditEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// makeJSONRPCBody encodes a JSON-RPC 2.0 request body.
func makeJSONRPCBody(method string, params interface{}) []byte {
	type req struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
		ID      int         `json:"id"`
	}
	body, _ := json.Marshal(req{JSONRPC: "2.0", Method: method, Params: params, ID: 1})
	return body
}

// makeToolsCallBody creates a tools/call request body for the given tool name.
func makeToolsCallBody(toolName string) []byte {
	return makeJSONRPCBody("tools/call", map[string]interface{}{
		"name":      toolName,
		"arguments": map[string]interface{}{},
	})
}

// sendRequest sends a JSON-RPC POST to the proxy server with optional auth.
func sendRequest(t *testing.T, proxyURL string, body []byte, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, proxyURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	return resp
}

// decodeJSONRPC reads and decodes a JSON-RPC response body.
func decodeJSONRPC(t *testing.T, body io.Reader) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}

// newAdminPipeline creates a proxy pipeline with:
//   - testAuthenticator accepting testToken as admin
//   - RBAC engine with default 3 roles
//   - Rate limiter at 1000 req/min
//   - testAuditLogger
//   - upstream pointing at the given mock server URL
func newAdminPipeline(t *testing.T, upstreamURL string) (*proxy.Pipeline, *testAuditLogger) {
	t.Helper()
	return newPipelineWithRole(t, upstreamURL, "admin")
}

// newPipelineWithRole creates a pipeline where the test token grants the given role.
func newPipelineWithRole(t *testing.T, upstreamURL string, role string) (*proxy.Pipeline, *testAuditLogger) {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{UpstreamURL: upstreamURL},
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	auth := &testAuthenticator{
		token: testToken,
		identity: &proxy.ClientIdentity{
			ClientID:  "test-client-1",
			Role:      role,
			SessionID: "test-session-1",
			Email:     "test@example.com",
		},
	}

	roles := []config.RoleConfig{
		{Name: "admin", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "readonly", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "restricted", AllowedTools: []string{"read_file", "list_dir"}, DenyTools: []string{}},
	}
	rbacEngine := rbac.NewEngine(roles)

	rlCfg := &config.RateLimitConfig{RequestsPerMinute: 1000, BurstSize: 100}
	rateLimiter := ratelimit.NewLimiter(rlCfg)

	auditLog := &testAuditLogger{}

	pipeline := proxy.NewPipelineForTest(handler, auth, rateLimiter, rbacEngine, nil, auditLog)
	return pipeline, auditLog
}

// newPipelineNoAuth creates a pipeline with NO authenticator (auth step skipped).
func newPipelineNoAuth(t *testing.T, upstreamURL string) *proxy.Pipeline {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{UpstreamURL: upstreamURL},
	}

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	roles := rbac.DefaultRoles()
	rbacEngine := rbac.NewEngine(roles)

	rlCfg := &config.RateLimitConfig{RequestsPerMinute: 1000, BurstSize: 100}
	rateLimiter := ratelimit.NewLimiter(rlCfg)

	return proxy.NewPipelineForTest(handler, nil, rateLimiter, rbacEngine, nil, nil)
}

// =============================================================================
// Server type tests — verify the proxy correctly forwards to each mock type
// =============================================================================

func TestProxy_ToolsOnlyServer(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	t.Run("tools/list", func(t *testing.T) {
		resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), testToken)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		body := decodeJSONRPC(t, resp.Body)
		result, ok := body["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected result object, got: %v", body)
		}
		tools, ok := result["tools"].([]interface{})
		if !ok {
			t.Fatalf("expected tools array in result, got: %v", result)
		}
		if len(tools) != 5 {
			t.Errorf("expected 5 tools, got %d", len(tools))
		}
	})

	t.Run("tools/call", func(t *testing.T) {
		resp := sendRequest(t, proxyServer.URL, makeToolsCallBody("read_file"), testToken)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		body := decodeJSONRPC(t, resp.Body)
		if _, hasResult := body["result"]; !hasResult {
			t.Errorf("expected result in response, got: %v", body)
		}
		if _, hasError := body["error"]; hasError {
			t.Errorf("unexpected error in response: %v", body["error"])
		}
	})
}

func TestProxy_ResourcesServer(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ResourcesServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL,
		makeJSONRPCBody("resources/read", map[string]string{"uri": "file:///project/README.md"}),
		testToken,
	)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := decodeJSONRPC(t, resp.Body)
	result, ok := body["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got: %v", body)
	}
	contents, ok := result["contents"].([]interface{})
	if !ok {
		t.Fatalf("expected contents array, got: %v", result)
	}
	if len(contents) == 0 {
		t.Error("expected at least one content item")
	}
}

func TestProxy_PromptsServer(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(PromptsServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL,
		makeJSONRPCBody("prompts/get", map[string]string{"name": "code_review"}),
		testToken,
	)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := decodeJSONRPC(t, resp.Body)
	result, ok := body["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got: %v", body)
	}
	messages, ok := result["messages"].([]interface{})
	if !ok {
		t.Fatalf("expected messages array, got: %v", result)
	}
	if len(messages) == 0 {
		t.Error("expected at least one message in prompt response")
	}
}

func TestProxy_MixedServer(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(MixedServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Test all method types in one session.
	methods := []struct {
		name   string
		method string
		params interface{}
	}{
		{"initialize", "initialize", map[string]interface{}{"protocolVersion": "2024-11-05", "clientInfo": map[string]string{"name": "test"}}},
		{"tools/list", "tools/list", nil},
		{"tools/call", "tools/call", map[string]interface{}{"name": "read_file", "arguments": map[string]interface{}{}}},
		{"resources/list", "resources/list", nil},
		{"resources/read", "resources/read", map[string]string{"uri": "file:///project/README.md"}},
		{"prompts/list", "prompts/list", nil},
		{"prompts/get", "prompts/get", map[string]string{"name": "code_review"}},
	}

	for _, m := range methods {
		m := m // capture
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody(m.method, m.params), testToken)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected 200 for %s, got %d: %s", m.method, resp.StatusCode, body)
			}

			result := decodeJSONRPC(t, resp.Body)
			if _, hasError := result["error"]; hasError {
				t.Errorf("unexpected error for %s: %v", m.method, result["error"])
			}
		})
	}
}

func TestProxy_SSEStreamingServer(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(SSEStreamingServer)
	t.Cleanup(mock.Close)

	// Build proxy pointing to the SSE mock server.
	// The mock server responds to all paths with SSE when a tools/call is sent.
	cfg := &config.Config{
		Server: config.ServerConfig{UpstreamURL: mock.URL()},
	}
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	auth := &testAuthenticator{
		token: testToken,
		identity: &proxy.ClientIdentity{
			ClientID: "sse-client",
			Role:     "admin",
			Email:    "sse@example.com",
		},
	}

	roles := []config.RoleConfig{
		{Name: "admin", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "readonly", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "restricted", AllowedTools: []string{}, DenyTools: []string{}},
	}
	rbacEngine := rbac.NewEngine(roles)
	rlCfg := &config.RateLimitConfig{RequestsPerMinute: 1000, BurstSize: 100}
	rateLimiter := ratelimit.NewLimiter(rlCfg)

	pipeline := proxy.NewPipelineForTest(handler, auth, rateLimiter, rbacEngine, nil, nil)
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Use a context with timeout to avoid hanging if SSE stream never ends.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqBody := makeToolsCallBody("long_running_tool")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyServer.URL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("create SSE request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("expected SSE content-type, got: %s", contentType)
	}

	// Read events and verify they arrive.
	scanner := bufio.NewScanner(resp.Body)
	var eventLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			eventLines = append(eventLines, line)
		}
		// Stop after reading enough events.
		if len(eventLines) >= 4 {
			break
		}
	}

	if len(eventLines) < 4 {
		t.Errorf("expected at least 4 SSE events (3 progress + 1 result), got %d", len(eventLines))
	}

	// Verify the last event is a result.
	if len(eventLines) > 0 {
		lastEvent := strings.TrimPrefix(eventLines[len(eventLines)-1], "data: ")
		var eventData map[string]interface{}
		if err := json.Unmarshal([]byte(lastEvent), &eventData); err != nil {
			t.Errorf("last event is not valid JSON: %v", err)
		} else if eventData["type"] != "result" {
			t.Errorf("expected last event type to be 'result', got: %v", eventData["type"])
		}
	}
}

// =============================================================================
// Auth tests
// =============================================================================

func TestProxy_NoAuthHeader_Returns401(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Send request with no Authorization header.
	resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	body := decodeJSONRPC(t, resp.Body)
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object in response, got: %v", body)
	}
	code, _ := errObj["code"].(float64)
	if code != -32001 {
		t.Errorf("expected JSON-RPC error code -32001 (Unauthorized), got %v", code)
	}
}

func TestProxy_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Send request with wrong token.
	resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), "wrong-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProxy_ValidToken_Proxies(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), testToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := decodeJSONRPC(t, resp.Body)
	if _, hasError := body["error"]; hasError {
		t.Errorf("unexpected error in response: %v", body["error"])
	}
}

// =============================================================================
// RBAC tests
// =============================================================================

func TestProxy_AdminRole_FullAccess(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	// Admin can call any tool.
	pipeline, _ := newPipelineWithRole(t, mock.URL(), "admin")
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Try execute_command — a privileged tool not in restricted's allowed list.
	resp := sendRequest(t, proxyServer.URL, makeToolsCallBody("execute_command"), testToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("admin should be able to call any tool, got %d: %s", resp.StatusCode, body)
	}
}

func TestProxy_ReadOnlyRole_DeniesToolsCall(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	// ReadOnly cannot call tools.
	pipeline, _ := newPipelineWithRole(t, mock.URL(), "readonly")
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL, makeToolsCallBody("read_file"), testToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403 for readonly role tools/call, got %d: %s", resp.StatusCode, body)
	}

	body := decodeJSONRPC(t, resp.Body)
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object in response, got: %v", body)
	}
	code, _ := errObj["code"].(float64)
	if code != -32002 {
		t.Errorf("expected JSON-RPC error code -32002 (Forbidden), got %v", code)
	}
}

func TestProxy_RestrictedRole_AllowedTool(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	// Restricted can call read_file and list_dir (in allowed_tools).
	pipeline, _ := newPipelineWithRole(t, mock.URL(), "restricted")
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL, makeToolsCallBody("read_file"), testToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("restricted role should be able to call read_file, got %d: %s", resp.StatusCode, body)
	}
}

func TestProxy_RestrictedRole_DeniedTool(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	// Restricted cannot call execute_command (not in allowed_tools).
	pipeline, _ := newPipelineWithRole(t, mock.URL(), "restricted")
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL, makeToolsCallBody("execute_command"), testToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("restricted role should be denied execute_command, got %d: %s", resp.StatusCode, body)
	}
}

func TestProxy_ToolsListFiltered(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	// Restricted role's tools/list should only return allowed tools.
	pipeline, _ := newPipelineWithRole(t, mock.URL(), "restricted")
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), testToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := decodeJSONRPC(t, resp.Body)
	result, ok := body["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object, got: %v", body)
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools array, got: %v", result)
	}

	// Restricted allowed_tools: ["read_file", "list_dir"] — only 2 of the 5 tools.
	if len(tools) != 2 {
		t.Errorf("expected 2 tools for restricted role, got %d: %v", len(tools), tools)
	}

	// Verify the names are correct.
	allowedNames := map[string]bool{"read_file": true, "list_dir": true}
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			t.Errorf("tool is not an object: %v", tool)
			continue
		}
		name, _ := toolMap["name"].(string)
		if !allowedNames[name] {
			t.Errorf("unexpected tool in filtered list: %s", name)
		}
	}
}

// =============================================================================
// Rate limit tests
// =============================================================================

func TestProxy_UnderRateLimit_Passes(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline, _ := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Send 5 requests well under the 100 req/min default.
	for i := range 5 {
		resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), testToken)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}
}

func TestProxy_OverRateLimit_Returns429(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	cfg := &config.Config{
		Server: config.ServerConfig{UpstreamURL: mock.URL()},
	}
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	auth := &testAuthenticator{
		token: testToken,
		identity: &proxy.ClientIdentity{
			ClientID: "rate-limited-client",
			Role:     "admin",
		},
	}

	roles := []config.RoleConfig{
		{Name: "admin", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "readonly", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "restricted", AllowedTools: []string{}, DenyTools: []string{}},
	}
	rbacEngine := rbac.NewEngine(roles)

	// Very tight rate limit: 1 req/min, burst of 1.
	rlCfg := &config.RateLimitConfig{RequestsPerMinute: 1, BurstSize: 1}
	rateLimiter := ratelimit.NewLimiter(rlCfg)

	pipeline := proxy.NewPipelineForTest(handler, auth, rateLimiter, rbacEngine, nil, nil)
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Send several requests. After consuming the burst, rate limit should kick in.
	var got429 bool
	for i := range 20 {
		resp := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), testToken)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
		_ = i
	}

	if !got429 {
		t.Error("expected at least one 429 response after exceeding tight rate limit")
	}
}

// =============================================================================
// Audit tests
// =============================================================================

func TestProxy_AuditLogWritten(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline, auditLog := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Allowed request.
	resp1 := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), testToken)
	resp1.Body.Close()

	// Denied request (no auth).
	resp2 := sendRequest(t, proxyServer.URL, makeJSONRPCBody("tools/list", nil), "")
	resp2.Body.Close()

	entries := auditLog.snapshot()
	if len(entries) != 2 {
		t.Errorf("expected 2 audit entries (1 allowed + 1 denied), got %d", len(entries))
	}

	// First should be allowed.
	if len(entries) > 0 && !entries[0].Allowed {
		t.Errorf("first request should be allowed, got denied: %s", entries[0].DeniedReason)
	}

	// Second should be denied.
	if len(entries) > 1 && entries[1].Allowed {
		t.Errorf("second request (no auth) should be denied")
	}
}

func TestProxy_AuditLogFormat(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline, auditLog := newAdminPipeline(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp := sendRequest(t, proxyServer.URL, makeToolsCallBody("read_file"), testToken)
	resp.Body.Close()

	entries := auditLog.snapshot()
	if len(entries) == 0 {
		t.Fatal("expected at least one audit entry")
	}

	entry := entries[0]

	// Verify required fields.
	if entry.RequestID == "" {
		t.Error("audit entry missing RequestID")
	}
	if entry.Timestamp.IsZero() {
		t.Error("audit entry missing Timestamp")
	}
	if entry.Method == "" {
		t.Error("audit entry missing Method")
	}
	if entry.ClientID == "" {
		t.Error("audit entry missing ClientID")
	}
	if entry.Latency <= 0 {
		t.Error("audit entry has zero/negative Latency")
	}
	if entry.Method == "tools/call" && entry.ToolName == "" {
		t.Error("audit entry for tools/call should have ToolName")
	}
	if !entry.Allowed {
		t.Errorf("audit entry for valid request should be Allowed=true, got DeniedReason: %s", entry.DeniedReason)
	}

	// Verify the entry serializes to valid JSON with required fields.
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("audit entry should marshal to JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("audit entry JSON should be parseable: %v", err)
	}

	requiredFields := []string{"timestamp", "client_id", "method", "allowed", "latency_ms", "request_id"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("audit entry JSON missing required field: %s", field)
		}
	}
}

// =============================================================================
// Pipeline tests
// =============================================================================

func TestProxy_HealthCheck(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline := newPipelineNoAuth(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	resp, err := http.Get(proxyServer.URL + "/health")
	if err != nil {
		t.Fatalf("health check request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /health, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("health response is not valid JSON: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got: %v", result["status"])
	}
}

// =============================================================================
// Batch RBAC integration test (HARD-03)
// =============================================================================

// makeBatchBody creates a JSON-RPC batch request body from items.
func makeBatchBody(items []struct {
	method string
	id     int
	tool   string
}) []byte {
	reqs := make([]map[string]interface{}, len(items))
	for i, item := range items {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      item.id,
			"method":  item.method,
		}
		if item.tool != "" {
			req["params"] = map[string]interface{}{
				"name":      item.tool,
				"arguments": map[string]interface{}{},
			}
		}
		reqs[i] = req
	}
	b, _ := json.Marshal(reqs)
	return b
}

// TestProxy_BatchRBAC_MixedAllowDeny sends a 3-item batch through the full pipeline where
// 2 items are allowed and 1 is denied, and verifies per-item RBAC enforcement.
func TestProxy_BatchRBAC_MixedAllowDeny(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	// Use "restricted" role: allowed_tools = [read_file, list_dir]
	pipeline, auditLog := newPipelineWithRole(t, mock.URL(), "restricted")
	proxyServer := httptest.NewServer(pipeline)
	t.Cleanup(proxyServer.Close)

	// Batch: item 1 = tools/list (allowed), item 2 = tools/call execute_command (denied),
	//        item 3 = tools/call read_file (allowed)
	body := makeBatchBody([]struct {
		method string
		id     int
		tool   string
	}{
		{"tools/list", 1, ""},
		{"tools/call", 2, "execute_command"},
		{"tools/call", 3, "read_file"},
	})

	req, err := http.NewRequest(http.MethodPost, proxyServer.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create batch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("batch request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for batch, got %d: %s", resp.StatusCode, body)
	}

	// Parse as JSON array.
	responseBytes, _ := io.ReadAll(resp.Body)
	var responses []map[string]interface{}
	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		t.Fatalf("expected JSON array response: %v — body: %s", err, responseBytes)
	}
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses in batch, got %d", len(responses))
	}

	// Item 1 (tools/list) should succeed.
	if _, hasError := responses[0]["error"]; hasError {
		t.Errorf("item 1 (tools/list) should succeed, got error: %v", responses[0]["error"])
	}

	// Item 2 (execute_command for restricted role) should be denied.
	if _, hasError := responses[1]["error"]; !hasError {
		t.Errorf("item 2 (execute_command, restricted role) should be denied, got: %v", responses[1])
	} else {
		errObj, _ := responses[1]["error"].(map[string]interface{})
		if errObj != nil {
			code, _ := errObj["code"].(float64)
			if code != -32002 { // ErrCodeForbidden
				t.Errorf("expected forbidden error code -32002 for denied item, got %v", code)
			}
		}
	}

	// Item 3 (read_file for restricted role) should succeed.
	if _, hasError := responses[2]["error"]; hasError {
		t.Errorf("item 3 (read_file) should succeed, got error: %v", responses[2]["error"])
	}

	// Verify audit log has 3 separate entries.
	time.Sleep(10 * time.Millisecond) // allow async audit writes to complete
	entries := auditLog.snapshot()
	if len(entries) != 3 {
		t.Errorf("expected 3 audit entries (one per batch item), got %d", len(entries))
	}

	// Verify the denied entry.
	var foundDenied bool
	for _, entry := range entries {
		if entry.Method == "tools/call" && entry.ToolName == "execute_command" {
			if entry.Allowed {
				t.Error("audit entry for denied execute_command should have Allowed=false")
			}
			foundDenied = true
		}
	}
	if !foundDenied {
		t.Error("no audit entry found for denied execute_command item")
	}
}

func TestProxy_GracefulShutdown(t *testing.T) {
	t.Parallel()

	mock := NewMockServer(ToolsOnlyServer)
	t.Cleanup(mock.Close)

	pipeline := newPipelineNoAuth(t, mock.URL())
	proxyServer := httptest.NewServer(pipeline)

	// Verify we can make requests before shutdown.
	resp, err := http.Get(proxyServer.URL + "/health")
	if err != nil {
		t.Fatalf("pre-shutdown health check failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("pre-shutdown: expected 200, got %d", resp.StatusCode)
	}

	// Shutdown and verify no panics.
	proxyServer.Close()

	// Verify server is no longer accepting connections after Close.
	_, err = http.Get(proxyServer.URL + "/health")
	if err == nil {
		t.Error("expected error after server close, got nil")
	}
}
