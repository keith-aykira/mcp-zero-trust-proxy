package proxy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/middleware"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
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

// helper: build a minimal resources/read JSON-RPC body
func resourcesReadBody(uri string) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params":  map[string]string{"uri": uri},
	})
	return b
}

// --- Tests ---

// TestPipelineNoMiddlewarePassesThrough tests that a pipeline with no middleware
// passes the request through to the upstream handler.
func TestPipelineNoMiddlewarePassesThrough(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil, nil)

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

	p := proxy.NewPipelineForTest(upstream, authFn, rlFn, rbacFn, nil, auditLogger)

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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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
	p1 := proxy.NewPipelineForTest(upstream, auth1, rl1, rbac1, nil, auditLogger)
	req1 := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("tool1")))
	p1.ServeHTTP(httptest.NewRecorder(), req1)

	// Request 2: denied (auth failure)
	auth2 := &mockAuthenticator{err: fmt.Errorf("invalid token")}
	p2 := proxy.NewPipelineForTest(upstream, auth2, rl1, rbac1, nil, auditLogger)
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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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
	p := proxy.NewPipelineForTest(upstream, nil, rl, rbac, nil, auditLogger)

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

// TestPipelineBodySizeLimit_Rejects verifies that requests over MaxBodySize are rejected with 413.
func TestPipelineBodySizeLimit_Rejects(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	// MaxBodySize of 10 bytes — any real request body will exceed this
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil, nil, proxy.WithMaxBodySize(10))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for oversized body, got %d", w.Code)
	}
	if upstream.called {
		t.Error("upstream should NOT be called when body exceeds limit")
	}
}

// TestPipelineBodySizeLimit_Allows verifies that requests under MaxBodySize pass through.
func TestPipelineBodySizeLimit_Allows(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	// MaxBodySize of 10000 bytes — enough for our test request
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil, nil, proxy.WithMaxBodySize(10000))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for normal-sized body, got %d", w.Code)
	}
	if !upstream.called {
		t.Error("upstream should be called when body is within limit")
	}
}

// TestPipelineRequestID_SuccessResponse verifies X-Request-ID is in every success response.
func TestPipelineRequestID_SuccessResponse(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID should be set on success responses")
	}
}

// TestPipelineRequestID_ErrorResponse verifies X-Request-ID is in error responses.
func TestPipelineRequestID_ErrorResponse(t *testing.T) {
	auth := &mockAuthenticator{err: fmt.Errorf("unauthorized")}
	upstream := &mockUpstreamHandler{}
	p := proxy.NewPipelineForTest(upstream, auth, nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID should be set on error responses too")
	}
}

// TestPipelineCORS_OptionsPreflightHandled verifies that OPTIONS preflight requests get a 204 with CORS headers.
func TestPipelineCORS_OptionsPreflightHandled(t *testing.T) {
	upstream := &mockUpstreamHandler{}
	corsConfig := &proxy.CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         3600,
	}
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil, nil, proxy.WithCORS(corsConfig))

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", w.Code)
	}
	if upstream.called {
		t.Error("upstream should NOT be called for OPTIONS preflight")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", w.Header().Get("Access-Control-Allow-Origin"), "https://example.com")
	}
}

// TestPipelineCORS_HeadersOnNonPreflightRequests verifies CORS headers are applied to all responses.
func TestPipelineCORS_HeadersOnNonPreflightRequests(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	corsConfig := &proxy.CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"POST"},
	}
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil, nil, proxy.WithCORS(corsConfig))

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", w.Header().Get("Access-Control-Allow-Origin"), "https://example.com")
	}
}

// TestPipelineCORS_NoCORSWhenNotConfigured verifies that CORS headers are absent when no config is set.
func TestPipelineCORS_NoCORSWhenNotConfigured(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	p := proxy.NewPipelineForTest(upstream, nil, nil, nil, nil, nil) // no CORS config

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("test")))
	req.Header.Set("Origin", "https://malicious.example.com")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("Access-Control-Allow-Origin should be absent when CORS not configured, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

// TestHandlerNoParsing verifies that Handler.ServeHTTP no longer parses the JSON-RPC body itself.
func TestHandlerNoParsing(t *testing.T) {
	// A handler that reads and returns the body it received
	var upstreamBodyReceived []byte
	upstreamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamBodyReceived = body
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamSrv.Close()

	// Handler should NOT set MCPRequestKey — that's Pipeline's job now
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","id":1,"params":{"name":"my_tool"}}`)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Registry: config.ServerRegistryConfig{
				Default: "default",
				Servers: []config.UpstreamServerConfig{
					{
						Name:    "default",
						URL:     upstreamSrv.URL,
						Enabled: true,
					},
				},
			},
		},
	}
	router, err := proxy.NewServerRouter(&cfg.Server.Registry)
	if err != nil {
		t.Fatalf("NewServerRouter error: %v", err)
	}
	h, err := proxy.NewHandler(cfg, router)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}
	h.ServeHTTP(rr, req)

	// Body should be forwarded intact
	if string(upstreamBodyReceived) != string(body) {
		t.Errorf("body not forwarded correctly: got %q, want %q", upstreamBodyReceived, body)
	}

	// The context should NOT have MCPRequestKey set by Handler (Pipeline sets it now)
	mcpReq := req.Context().Value(proxy.MCPRequestKey)
	if mcpReq != nil {
		t.Error("Handler should NOT set MCPRequestKey in context — that is Pipeline's responsibility")
	}
}

// =============================================================================
// Batch JSON-RPC per-item RBAC enforcement tests (HARD-03)
// =============================================================================

// batchBody builds a JSON-RPC batch request body from a list of (method, toolName) pairs.
// toolName is ignored for non-tools/call methods.
func batchBody(items []struct{ method, tool string }) []byte {
	reqs := make([]map[string]interface{}, len(items))
	for i, item := range items {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      i + 1,
			"method":  item.method,
		}
		if item.tool != "" {
			req["params"] = map[string]string{"name": item.tool}
		}
		reqs[i] = req
	}
	b, _ := json.Marshal(reqs)
	return b
}

// TestBatchRBAC_MixedAllowDeny verifies that in a batch with 3 items where item 2
// is denied, the response array contains items 1 and 3 as successes and item 2 as a JSON-RPC error.
func TestBatchRBAC_MixedAllowDeny(t *testing.T) {
	// RBAC: allow tools/list and read_file, deny execute_command
	processCount := 0
	rbacEngine := &perItemRBAC{
		allow: func(req *proxy.MCPRequest) bool {
			processCount++
			if req.Method == "tools/call" {
				name := ""
				var p struct{ Name string `json:"name"` }
				json.Unmarshal(req.Params, &p) //nolint:errcheck
				name = p.Name
				return name != "execute_command"
			}
			return true
		},
	}

	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "u1", Role: "restricted"}}
	rl := &mockRateLimiter{allow: true}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbacEngine, nil, auditLogger)

	// 3-item batch: tools/list (allowed), tools/call execute_command (denied), tools/call read_file (allowed)
	body := batchBody([]struct{ method, tool string }{
		{"tools/list", ""},
		{"tools/call", "execute_command"},
		{"tools/call", "read_file"},
	})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for batch response, got %d: %s", w.Code, w.Body.String())
	}

	// Parse the response as a JSON array
	var responses []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &responses); err != nil {
		t.Fatalf("batch response should be a JSON array, got: %s", w.Body.String())
	}
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses in batch, got %d", len(responses))
	}

	// Item 2 (index 1) should be an error
	if _, hasError := responses[1]["error"]; !hasError {
		t.Errorf("item 2 (execute_command) should have been denied (error), got: %v", responses[1])
	}
	errObj, _ := responses[1]["error"].(map[string]interface{})
	if errObj != nil {
		code, _ := errObj["code"].(float64)
		if code != float64(proxy.ErrCodeForbidden) {
			t.Errorf("expected error code %d for denied item, got %v", proxy.ErrCodeForbidden, code)
		}
	}
}

// TestBatchRBAC_AllAllowed verifies all items are forwarded when all are allowed.
func TestBatchRBAC_AllAllowed(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "u1", Role: "admin"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{} // no errors — all allowed
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	body := batchBody([]struct{ method, tool string }{
		{"tools/list", ""},
		{"tools/call", "read_file"},
		{"tools/call", "list_dir"},
	})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &responses); err != nil {
		t.Fatalf("expected JSON array response: %v — body: %s", err, w.Body.String())
	}
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	for i, resp := range responses {
		if _, hasError := resp["error"]; hasError {
			t.Errorf("item %d should be allowed but got error: %v", i+1, resp["error"])
		}
	}
}

// TestBatchRBAC_AllDenied verifies all items return error responses when all are denied.
func TestBatchRBAC_AllDenied(t *testing.T) {
	upstream := &mockUpstreamHandler{}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "u1", Role: "readonly"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{processErr: fmt.Errorf("rbac: method denied")} // deny all
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	body := batchBody([]struct{ method, tool string }{
		{"tools/call", "read_file"},
		{"tools/call", "execute_command"},
	})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for batch error array, got %d", w.Code)
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &responses); err != nil {
		t.Fatalf("expected JSON array response: %v — body: %s", err, w.Body.String())
	}
	if len(responses) != 2 {
		t.Fatalf("expected 2 error responses, got %d", len(responses))
	}
	for i, resp := range responses {
		if _, hasError := resp["error"]; !hasError {
			t.Errorf("item %d should be denied but got no error: %v", i+1, resp)
		}
	}
	// Upstream should NOT have been called since all items were denied
	if upstream.called {
		t.Error("upstream should NOT be called when all batch items are denied")
	}
}

// TestBatchRBAC_SingleRequestUnchanged verifies single (non-batch) request behavior is unchanged.
func TestBatchRBAC_SingleRequestUnchanged(t *testing.T) {
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "u1", Role: "admin"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("read_file")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("single request should still work, got %d", w.Code)
	}
	if !upstream.called {
		t.Error("upstream should be called for single allowed request")
	}
	// Response should NOT be a JSON array
	var arr []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err == nil {
		t.Error("single request should return object, not array")
	}
}

// TestBatchRBAC_AuditPerItem verifies that each batch item gets its own audit entry.
func TestBatchRBAC_AuditPerItem(t *testing.T) {
	rbacEngine := &perItemRBAC{
		allow: func(req *proxy.MCPRequest) bool {
			if req.Method == "tools/call" {
				var p struct{ Name string `json:"name"` }
				json.Unmarshal(req.Params, &p) //nolint:errcheck
				return p.Name != "execute_command"
			}
			return true
		},
	}
	upstream := &mockUpstreamHandler{responseBody: `{"jsonrpc":"2.0","id":1,"result":{}}`, statusCode: 200}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "u1", Role: "admin"}}
	rl := &mockRateLimiter{allow: true}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbacEngine, nil, auditLogger)

	body := batchBody([]struct{ method, tool string }{
		{"tools/list", ""},
		{"tools/call", "execute_command"},
		{"tools/call", "read_file"},
	})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	entries := auditLogger.entries
	if len(entries) != 3 {
		t.Fatalf("expected 3 audit entries (one per batch item), got %d", len(entries))
	}
	// Item 2 should be denied
	if entries[1].Allowed {
		t.Error("audit entry for item 2 (execute_command) should have Allowed=false")
	}
	if entries[1].DeniedReason == "" {
		t.Error("denied audit entry should have DeniedReason set")
	}
	// Items 1 and 3 should be allowed
	if !entries[0].Allowed {
		t.Error("audit entry for item 1 (tools/list) should have Allowed=true")
	}
	if !entries[2].Allowed {
		t.Error("audit entry for item 3 (read_file) should have Allowed=true")
	}
}

// TestBatchRBAC_EmptyBatch verifies empty batch returns empty array response.
func TestBatchRBAC_EmptyBatch(t *testing.T) {
	upstream := &mockUpstreamHandler{}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "u1", Role: "admin"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("[]")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	// Should return 200 with empty array
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &responses); err != nil {
		t.Fatalf("expected JSON array: %v — body: %s", err, w.Body.String())
	}
	if len(responses) != 0 {
		t.Errorf("expected empty array, got %d items", len(responses))
	}
}

// perItemRBAC is a mock RBAC engine where each Process call uses a custom allow func.
type perItemRBAC struct {
	allow func(req *proxy.MCPRequest) bool
}

func (r *perItemRBAC) Process(ctx context.Context, req *proxy.MCPRequest, identity *proxy.ClientIdentity) (*proxy.MCPRequest, error) {
	if r.allow(req) {
		return req, nil
	}
	return nil, fmt.Errorf("rbac: method denied for role")
}

func (r *perItemRBAC) FilterToolsList(identity *proxy.ClientIdentity, toolsList json.RawMessage) (json.RawMessage, error) {
	return toolsList, nil
}

// =============================================================================
// SSE role enforcement tests (security fix: restrict SSE to admin role only)
// =============================================================================

// TestSSERoleEnforcement_AdminAllowed verifies that a client with the admin role
// can make SSE requests (i.e., the pipeline does NOT return 403 for admin).
func TestSSERoleEnforcement_AdminAllowed(t *testing.T) {
	// Upstream returns a simple SSE stream for the test; the pipeline should let it through.
	upstream := &mockUpstreamHandler{responseBody: "", statusCode: 200}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "admin-client", Role: "admin"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	// Admin should NOT receive a 403 — the pipeline passes through to upstream.
	if w.Code == http.StatusForbidden {
		t.Errorf("admin role should be allowed SSE, got 403: %s", w.Body.String())
	}
}

// TestSSERoleEnforcement_ReadonlyDenied verifies that a client with the readonly
// role receives 403 Forbidden when attempting an SSE request.
func TestSSERoleEnforcement_ReadonlyDenied(t *testing.T) {
	upstream := &mockUpstreamHandler{}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "readonly-client", Role: "readonly"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("readonly role should get 403 for SSE, got %d", w.Code)
	}
	if upstream.called {
		t.Error("upstream should NOT be called when SSE is denied")
	}
	// Verify the response contains a meaningful error message.
	body := w.Body.String()
	if !strings.Contains(body, "SSE streaming requires admin role") {
		t.Errorf("expected error message about SSE requiring admin role, got: %s", body)
	}
}

// TestSSERoleEnforcement_RestrictedDenied verifies that a client with the restricted
// role also receives 403 Forbidden when attempting an SSE request.
func TestSSERoleEnforcement_RestrictedDenied(t *testing.T) {
	upstream := &mockUpstreamHandler{}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "restricted-client", Role: "restricted"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("restricted role should get 403 for SSE, got %d", w.Code)
	}
	if upstream.called {
		t.Error("upstream should NOT be called when SSE is denied for restricted role")
	}
}

// TestSSERoleEnforcement_NoIdentityDenied verifies that an unauthenticated request
// (no identity) also cannot access SSE.
func TestSSERoleEnforcement_NoIdentityDenied(t *testing.T) {
	upstream := &mockUpstreamHandler{}
	// nil auth means no identity is set
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, nil, rl, rbac, nil, auditLogger)

	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("unauthenticated SSE request should get 403, got %d", w.Code)
	}
	if upstream.called {
		t.Error("upstream should NOT be called when SSE is denied for unauthenticated client")
	}
}

// TestSSERoleEnforcement_DeniedAudited verifies that denied SSE attempts are logged.
func TestSSERoleEnforcement_DeniedAudited(t *testing.T) {
	upstream := &mockUpstreamHandler{}
	auth := &mockAuthenticator{identity: &proxy.ClientIdentity{ClientID: "readonly-client", Role: "readonly"}}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

	req := httptest.NewRequest("GET", "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if len(auditLogger.entries) == 0 {
		t.Fatal("expected an audit entry for denied SSE attempt")
	}
	entry := auditLogger.entries[0]
	if entry.Allowed {
		t.Error("denied SSE audit entry should have Allowed=false")
	}
	if entry.DeniedReason == "" {
		t.Error("denied SSE audit entry should have DeniedReason set")
	}
	if entry.ClientID != "readonly-client" {
		t.Errorf("audit entry ClientID should be %q, got %q", "readonly-client", entry.ClientID)
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

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, nil, auditLogger)

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

// mockPIIMasker simulates PII masking for tests.
type mockPIIMasker struct {
	called         bool
	toolName       string
	shouldMask     bool
	maskRequest    func(req *proxy.MCPRequest) (*proxy.MCPRequest, error)
	maskResponse   func(method string, toolOrResource string, response json.RawMessage) (json.RawMessage, error)
}

func (m *mockPIIMasker) ShouldMaskTool(toolName string) bool {
	return m.shouldMask
}

func (m *mockPIIMasker) MaskRequest(req *proxy.MCPRequest) (*proxy.MCPRequest, error) {
	m.called = true
	if m.maskRequest != nil {
		return m.maskRequest(req)
	}
	return req, nil
}

func (m *mockPIIMasker) MaskResponse(method string, toolOrResource string, response json.RawMessage) (json.RawMessage, error) {
	m.called = true
	m.toolName = toolOrResource
	if !m.shouldMask {
		return response, nil
	}
	if m.maskResponse != nil {
		return m.maskResponse(method, toolOrResource, response)
	}
	return response, nil
}

// TestPipelinePIIResponseMasking tests that PII masking is applied to tools/call responses.
func TestPipelinePIIResponseMasking(t *testing.T) {
	upstream := &mockUpstreamHandler{
		responseBody: `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"email: test@example.com"}]}}`,
		statusCode:   200,
	}
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "admin"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	piiMasker := &mockPIIMasker{
		shouldMask: true, // Simulates tool assigned to sensitivity class
		maskResponse: func(method string, toolName string, response json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"email: ***REDACTED***"}]}}`), nil
		},
	}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, piiMasker, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("get_user")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if !piiMasker.called {
		t.Error("PII masker MaskResponse should have been called")
	}
	if piiMasker.toolName != "get_user" {
		t.Errorf("expected tool name 'get_user', got '%s'", piiMasker.toolName)
	}

	body := w.Body.String()
	if !strings.Contains(body, "***REDACTED***") {
		t.Errorf("response should contain masked data, got: %s", body)
	}
	if strings.Contains(body, "test@example.com") {
		t.Error("response should not contain unmasked email")
	}
}

// TestPipelinePIIResponseMaskingResourcesRead tests PII masking on resources/read responses.
func TestPipelinePIIResponseMaskingResourcesRead(t *testing.T) {
	upstream := &mockUpstreamHandler{
		responseBody: `{"jsonrpc":"2.0","id":1,"result":{"contents":[{"uri":"file:///secret","text":"password: secret123"}]}}`,
		statusCode:   200,
	}
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "admin"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	piiMasker := &mockPIIMasker{
		shouldMask: true, // Simulates resource assigned to sensitivity class
		maskResponse: func(method string, resourceURI string, response json.RawMessage) (json.RawMessage, error) {
			if resourceURI != "file:///secret" {
				return nil, fmt.Errorf("expected resource URI 'file:///secret', got '%s'", resourceURI)
			}
			return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"contents":[{"uri":"file:///secret","text":"password: ***REDACTED***"}]}}`), nil
		},
	}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, piiMasker, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(resourcesReadBody("file:///secret")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if !piiMasker.called {
		t.Error("PII masker MaskResponse should have been called for resources/read")
	}

	body := w.Body.String()
	if !strings.Contains(body, "***REDACTED***") {
		t.Errorf("response should contain masked data, got: %s", body)
	}
	if strings.Contains(body, "secret123") {
		t.Error("response should not contain unmasked password")
	}
}

// TestPipelinePIINoMaskingForUnassignedTool tests that PII masking is skipped for unassigned tools.
func TestPipelinePIINoMaskingForUnassignedTool(t *testing.T) {
	unmaskedResponse := `{"jsonrpc":"2.0","id":1,"result":{"message":"hello","email":"test@example.com"}}`
	upstream := &mockUpstreamHandler{
		responseBody: unmaskedResponse,
		statusCode:   200,
	}
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "admin"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{}
	piiMasker := &mockPIIMasker{
		shouldMask: false, // Simulates unassigned tool returning no patterns
		maskResponse: func(method string, toolOrResource string, response json.RawMessage) (json.RawMessage, error) {
			// When tool is unassigned, masker returns response unchanged
			return response, nil
		},
	}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, piiMasker, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsCallBody("unassigned_tool")))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	body := w.Body.String()
	// Response should be passed through unchanged for unassigned tools
	if !strings.Contains(body, `"email":"test@example.com"`) {
		t.Errorf("response should be unchanged for unassigned tool, got: %s", body)
	}
	if strings.Contains(body, "***REDACTED***") {
		t.Error("response should not contain redacted content for unassigned tool")
	}
}

// TestPipelinePIIWithRBACFiltering tests that both PII masking and RBAC filtering can work together.
func TestPipelinePIIWithRBACFiltering(t *testing.T) {
	upstream := &mockUpstreamHandler{
		responseBody: toolsListResponse([]string{"tool1", "tool2", "admin_tool"}),
		statusCode:   200,
	}
	auth := &mockAuthenticator{
		identity: &proxy.ClientIdentity{ClientID: "user1", Role: "readonly"},
	}
	rl := &mockRateLimiter{allow: true}
	rbac := &mockRBACEngine{
		filteredResult: json.RawMessage(`{"tools":[{"name":"tool1"},{"name":"tool2"}]}`),
	}
	piiMasker := &mockPIIMasker{}
	auditLogger := &mockAuditLogger{}

	p := proxy.NewPipelineForTest(upstream, auth, rl, rbac, piiMasker, auditLogger)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(toolsListBody()))
	w := httptest.NewRecorder()

	p.ServeHTTP(w, req)

	if !rbac.filterCalled {
		t.Error("RBAC filtering should be called for tools/list")
	}

	body := w.Body.String()
	if strings.Contains(body, "admin_tool") {
		t.Error("RBAC should have filtered out admin_tool")
	}
	if !strings.Contains(body, "tool1") || !strings.Contains(body, "tool2") {
		t.Errorf("RBAC should have kept tool1 and tool2, got: %s", body)
	}
}
