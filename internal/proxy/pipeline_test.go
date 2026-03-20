package proxy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/middleware"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
)

// --- Mock implementations ---

// mockAuthenticator tracks calls and returns configured values.
type mockAuthenticator struct {
	called   bool
	identity *proxy.ClientIdentity
	err      error
}

func (m *mockAuthenticator) Authenticate(r *http.Request) (*proxy.ClientIdentity, error) {
	m.called = true
	return m.identity, m.err
}

// mockRateLimiter tracks calls and returns configured values.
type mockRateLimiter struct {
	called   bool
	clientID string
	allow    bool
}

func (m *mockRateLimiter) Allow(clientID string) bool {
	m.called = true
	m.clientID = clientID
	return m.allow
}

// mockRBACEngine tracks calls and returns configured values.
type mockRBACEngine struct {
	processCalled  bool
	filterCalled   bool
	processErr     error
	filteredResult json.RawMessage
}

func (m *mockRBACEngine) Process(ctx context.Context, req *proxy.MCPRequest, identity *proxy.ClientIdentity) (*proxy.MCPRequest, error) {
	m.processCalled = true
	return req, m.processErr
}

func (m *mockRBACEngine) FilterToolsList(identity *proxy.ClientIdentity, toolsList json.RawMessage) (json.RawMessage, error) {
	m.filterCalled = true
	if m.filteredResult != nil {
		return m.filteredResult, nil
	}
	return toolsList, nil
}

// mockAuditLogger captures logged audit entries.
type mockAuditLogger struct {
	entries []middleware.AuditEntry
}

func (m *mockAuditLogger) Log(entry middleware.AuditEntry) {
	m.entries = append(m.entries, entry)
}

// mockUpstreamHandler simulates the upstream MCP server.
type mockUpstreamHandler struct {
	called       bool
	responseBody string
	statusCode   int
}

func (m *mockUpstreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.called = true
	if m.statusCode != 0 {
		w.WriteHeader(m.statusCode)
	}
	if m.responseBody != "" {
		w.Write([]byte(m.responseBody)) //nolint:errcheck
	}
}

// helper: build a minimal tools/call JSON-RPC body
func toolsCallBody(tool string) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]string{"name": tool},
	})
	return b
}

// helper: build a minimal tools/list JSON-RPC body
func toolsListBody() []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	return b
}

// helper: build a minimal tools/list response body from upstream
func toolsListResponse(tools []string) string {
	toolEntries := make([]map[string]string, len(tools))
	for i, t := range tools {
		toolEntries[i] = map[string]string{"name": t, "description": ""}
	}
	result, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"tools": toolEntries,
		},
	})
	return string(result)
}

// --- Tests ---

// TestPipelineNoMiddlewarePassesThrough tests that a pipeline with no middleware
// passes the request through to the upstream handler.
func TestPipelineNoMiddlewarePassesThrough(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if !upstream.called {
		t.Error("upstream was not called when pipeline has no middleware")
	}
}

// TestPipelineMiddlewareOrder tests that middleware executes in order: auth -> rate limit -> RBAC.
func TestPipelineMiddlewareOrder(t *testing.T) {
	callOrder := []string{}
	step := 0

	authFn := &callOrderAuth{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "admin"},
		onCall: func() {
			step++
			callOrder = append(callOrder, fmt.Sprintf("auth:%d", step))
		},
	}
	rlFn := &callOrderRL{
		allow: true,
		onCall: func() {
			step++
			callOrder = append(callOrder, fmt.Sprintf("ratelimit:%d", step))
		},
	}
	rbacFn := &callOrderRBAC{
		onCall: func() {
			step++
			callOrder = append(callOrder, fmt.Sprintf("rbac:%d", step))
		},
	}

	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, authFn, rlFn, rbacFn, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if len(callOrder) != 3 {
		t.Fatalf("expected 3 middleware calls, got %d: %v", len(callOrder), callOrder)
	}
	// Verify order: auth=1, ratelimit=2, rbac=3
	if !strings.HasSuffix(callOrder[0], ":1") {
		t.Errorf("first call should be auth at step 1, got %q", callOrder[0])
	}
	if !strings.HasSuffix(callOrder[1], ":2") {
		t.Errorf("second call should be ratelimit at step 2, got %q", callOrder[1])
	}
	if !strings.HasSuffix(callOrder[2], ":3") {
		t.Errorf("third call should be rbac at step 3, got %q", callOrder[2])
	}
	if !strings.Contains(callOrder[0], "auth") {
		t.Errorf("first call should be auth, got %q", callOrder[0])
	}
	if !strings.Contains(callOrder[1], "ratelimit") {
		t.Errorf("second call should be ratelimit, got %q", callOrder[1])
	}
	if !strings.Contains(callOrder[2], "rbac") {
		t.Errorf("third call should be rbac, got %q", callOrder[2])
	}
}

// callOrderAuth tracks when Authenticate is called.
type callOrderAuth struct {
	identity *proxy.ClientIdentity
	err      error
	onCall   func()
}

func (c *callOrderAuth) Authenticate(r *http.Request) (*proxy.ClientIdentity, error) {
	c.onCall()
	return c.identity, c.err
}

// callOrderRL tracks when Allow is called.
type callOrderRL struct {
	allow  bool
	onCall func()
}

func (c *callOrderRL) Allow(clientID string) bool {
	c.onCall()
	return c.allow
}

// callOrderRBAC tracks when Process is called.
type callOrderRBAC struct {
	err    error
	onCall func()
}

func (c *callOrderRBAC) Process(ctx context.Context, req *proxy.MCPRequest, identity *proxy.ClientIdentity) (*proxy.MCPRequest, error) {
	c.onCall()
	return req, c.err
}

func (c *callOrderRBAC) FilterToolsList(identity *proxy.ClientIdentity, toolsList json.RawMessage) (json.RawMessage, error) {
	return toolsList, nil
}

// TestPipelineAuthFailureShortCircuits tests that auth failure prevents RBAC and rate limit from being called.
func TestPipelineAuthFailureShortCircuits(t *testing.T) {
	auth := &mockAuthenticator{err: fmt.Errorf("invalid token")}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	req.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if rl.called {
		t.Error("rate limiter should NOT be called when auth fails")
	}
	if rbac.processCalled {
		t.Error("RBAC should NOT be called when auth fails")
	}
	if upstream.called {
		t.Error("upstream should NOT be called when auth fails")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestPipelineRateLimitFailureShortCircuits tests that rate limit failure prevents RBAC from being called.
func TestPipelineRateLimitFailureShortCircuits(t *testing.T) {
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "admin"},
	}
	rl := &mockRateLimiter{allow: false}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if rbac.processCalled {
		t.Error("RBAC should NOT be called when rate limit fails")
	}
	if upstream.called {
		t.Error("upstream should NOT be called when rate limit fails")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

// TestPipelineRBACDenialBlocksUpstream tests that RBAC denial prevents the request from reaching upstream.
func TestPipelineRBACDenialBlocksUpstream(t *testing.T) {
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "readonly"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{processErr: fmt.Errorf("rbac: method denied")}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if upstream.called {
		t.Error("upstream should NOT be called when RBAC denies")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestPipelineSuccessAuditAllowed tests that successful requests produce audit entry with allowed=true.
func TestPipelineSuccessAuditAllowed(t *testing.T) {
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "admin", SessionID: "sess1"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("my_tool")))
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if len(auditLogger.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditLogger.entries))
	}
	entry := auditLogger.entries[0]
	if !entry.Allowed {
		t.Error("audit entry should have Allowed=true for successful request")
	}
	if entry.ClientID != "user1" {
		t.Errorf("audit entry ClientID: got %q, want %q", entry.ClientID, "user1")
	}
	if entry.Method != "tools/call" {
		t.Errorf("audit entry Method: got %q, want %q", entry.Method, "tools/call")
	}
	if entry.DeniedReason != "" {
		t.Errorf("audit entry DeniedReason should be empty for allowed request, got %q", entry.DeniedReason)
	}
	if entry.Latency < 0 {
		t.Errorf("audit entry Latency should not be negative, got %v", entry.Latency)
	}
	if entry.RequestID == "" {
		t.Error("audit entry RequestID should be set")
	}
}

// TestPipelineFailureAuditDenied tests that denied requests produce audit entry with allowed=false and denied_reason.
func TestPipelineFailureAuditDenied(t *testing.T) {
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "readonly"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{processErr: fmt.Errorf("rbac: method denied for role readonly")}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("my_tool")))
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if len(auditLogger.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditLogger.entries))
	}
	entry := auditLogger.entries[0]
	if entry.Allowed {
		t.Error("audit entry should have Allowed=false for denied request")
	}
	if entry.DeniedReason == "" {
		t.Error("audit entry DeniedReason should be set for denied request")
	}
}

// TestPipelineAuditBothAllowedAndDenied verifies audit logging happens for both outcomes.
func TestPipelineAuditBothAllowedAndDenied(t *testing.T) {
	identity := &proxy.ClientIdentity{ClientID: "user1", Role: "admin", SessionID: "sess1"}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}

	// Request 1: allowed
	auth1 := &mockAuthenticator{identity: identity}
	rl1 := &mockRateLimiter{allow: true}
	rbac1 := &mockRBACEngine{}
	p1 := proxy.NewPipelineForTest(upstream, auth1, rl1, rbac1, auditLogger)
	req1 := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("tool1")))
	p1.ServeHTTP(httptest.NewRecorder(), req1)

	// Request 2: denied (auth failure)
	auth2 := &mockAuthenticator{err: fmt.Errorf("invalid token")}
	p2 := proxy.NewPipelineForTest(upstream, auth2, rl1, rbac1, auditLogger)
	req2 := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("tool2")))
	p2.ServeHTTP(httptest.NewRecorder(), req2)

	if len(auditLogger.entries) != 2 {
		t.Fatalf("expected 2 audit entries (one allowed, one denied), got %d", len(auditLogger.entries))
	}
	if !auditLogger.entries[0].Allowed {
		t.Error("first entry should be allowed")
	}
	if auditLogger.entries[1].Allowed {
		t.Error("second entry should be denied")
	}
}

// TestPipelineToolsListFiltering tests that tools/list responses are filtered through RBAC.
func TestPipelineToolsListFiltering(t *testing.T) {
	filteredResult := json.RawMessage(`{"tools":[{"name":"safe_tool"}]}`)
	upstream := &mockUpstreamHandler{
		responseBody: toolsListResponse([]string{"safe_tool", "admin_tool"}),
		statusCode:   200,
	}
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "restricted"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{
		filteredResult: filteredResult,
	}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsListBody()))
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if !rbac.filterCalled {
		t.Error("RBAC FilterToolsList should have been called for tools/list response")
	}

	body := w.Body.String()
	if !strings.Contains(body, "safe_tool") {
		t.Errorf("response should contain safe_tool, got: %s", body)
	}
}

// TestPipelineHealthBypassesPipeline tests that /health route bypasses all middleware.
func TestPipelineHealthBypassesPipeline(t *testing.T) {
	auth := &mockAuthenticator{err: fmt.Errorf("invalid token")} // auth would fail if called
	rl := &mockRateLimiter{allow: false}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health check: expected 200, got %d", w.Code)
	}
	if auth.called {
		t.Error("auth should NOT be called for /health")
	}
	if rl.called {
		t.Error("rate limiter should NOT be called for /health")
	}
	if upstream.called {
		t.Error("upstream should NOT be called for /health")
	}
	if len(auditLogger.entries) != 0 {
		t.Error("audit logger should NOT be called for /health")
	}
}

// TestPipelineAuthRoutesBypassPipeline tests that /auth/* routes bypass the pipeline.
func TestPipelineAuthRoutesBypassPipeline(t *testing.T) {
	auth := &mockAuthenticator{err: fmt.Errorf("invalid token")} // auth would fail if called
	rl := &mockRateLimiter{allow: false}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("GET", "/auth/start", nil)
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	// Should not call rate limiter or RBAC (auth route is handled separately)
	if rl.called {
		t.Error("rate limiter should NOT be called for /auth/start")
	}
	if rbac.processCalled {
		t.Error("RBAC should NOT be called for /auth/start")
	}
	if upstream.called {
		t.Error("upstream should NOT be called for /auth/start")
	}
}

// TestPipelineNoAuthNilSkipsAuthentication verifies nil auth skips the auth step.
func TestPipelineNoAuthNilSkipsAuthentication(t *testing.T) {
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}

	// nil auth = no authentication step
	p := proxy.NewPipelineForTest(upstream, nil, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if !upstream.called {
		t.Error("upstream should be called when auth is nil")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestPipelineLatencyTracking verifies latency is recorded in audit entries.
func TestPipelineLatencyTracking(t *testing.T) {
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "admin"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if len(auditLogger.entries) == 0 {
		t.Fatal("no audit entries recorded")
	}

	entry := auditLogger.entries[0]
	// Latency should be non-negative and under 1 second for a unit test
	if entry.Latency < 0 {
		t.Errorf("latency should not be negative, got %v", entry.Latency)
	}
	if entry.Latency > 1*time.Second {
		t.Errorf("latency should be under 1 second in unit test, got %v", entry.Latency)
	}
}
