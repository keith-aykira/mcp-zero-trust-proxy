// Package e2e contains end-to-end persona validation tests.
// Each persona exercises the full proxy pipeline with real HTTP requests
// against a live proxy backed by a mock MCP server.
//
// Run all personas: go test ./tests/e2e/ -v -timeout 120s
// Run one persona:  go test ./tests/e2e/ -v -run TestMarcus
package e2e

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/license"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/rbac"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/ratelimit"
)

// --- Persona token constants ---

const (
	tokenMarcus         = "marcus-token-e2e"
	tokenPriyaAdmin     = "priya-admin-token-e2e"
	tokenPriyaIntern    = "priya-intern-token-e2e"
	tokenJames          = "james-token-e2e"
	tokenSofiaAttorney  = "sofia-attorney-token-e2e"
	tokenSofiaParalegal = "sofia-paralegal-token-e2e"
	tokenKai            = "kai-token-e2e"
	tokenUnknown        = "unknown-invalid-token"
)

// personaIdentity maps Bearer tokens to client identities.
// This simulates what a real OAuth provider + user-role mapping would produce.
var personaIdentity = map[string]*proxy.ClientIdentity{
	tokenMarcus:         {ClientID: "marcus", Email: "marcus@test.local", Role: "admin", SessionID: "sess-marcus"},
	tokenPriyaAdmin:     {ClientID: "priya", Email: "priya@startup.io", Role: "admin", SessionID: "sess-priya"},
	tokenPriyaIntern:    {ClientID: "intern", Email: "intern@startup.io", Role: "readonly", SessionID: "sess-intern"},
	tokenJames:          {ClientID: "james", Email: "james@bigcorp.com", Role: "admin", SessionID: "sess-james"},
	tokenSofiaAttorney:  {ClientID: "attorney", Email: "attorney@lawfirm.com", Role: "admin", SessionID: "sess-attorney"},
	tokenSofiaParalegal: {ClientID: "paralegal", Email: "paralegal@lawfirm.com", Role: "readonly", SessionID: "sess-paralegal"},
	tokenKai:            {ClientID: "kai", Email: "kai@seriesb.io", Role: "admin", SessionID: "sess-kai"},
}

// multiTokenAuth accepts any token in personaIdentity and returns its ClientIdentity.
type multiTokenAuth struct{}

func (a *multiTokenAuth) Authenticate(r *http.Request) (*proxy.ClientIdentity, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("Authorization header must use Bearer scheme")
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	identity, ok := personaIdentity[token]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	return identity, nil
}

// --- Audit logger that captures entries in memory ---

type memAuditLogger struct {
	mu      sync.Mutex
	entries []proxy.AuditEntry
}

func (l *memAuditLogger) Log(entry proxy.AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *memAuditLogger) snapshot() []proxy.AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]proxy.AuditEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

func (l *memAuditLogger) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = nil
}

// --- Mock MCP server ---

// mockMCPServer creates a mock upstream that handles all MCP JSON-RPC methods.
// If delay > 0, tools/call responses are delayed (for graceful shutdown testing).
func mockMCPServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
			ID      interface{}     `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSONRPC(w, nil, map[string]interface{}{"code": -32700, "message": "Parse error"}, req.ID)
			return
		}

		switch req.Method {
		case "initialize":
			writeJSONRPC(w, map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":   map[string]interface{}{"tools": map[string]bool{"listChanged": false}},
				"serverInfo":     map[string]interface{}{"name": "mock-mcp", "version": "1.0.0"},
			}, nil, req.ID)

		case "tools/list":
			writeJSONRPC(w, map[string]interface{}{
				"tools": []map[string]interface{}{
					{"name": "read_file", "description": "Read a file", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]string{"type": "string"}}}},
					{"name": "write_file", "description": "Write a file", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]string{"type": "string"}, "content": map[string]string{"type": "string"}}}},
					{"name": "list_dir", "description": "List directory", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"path": map[string]string{"type": "string"}}}},
					{"name": "search", "description": "Search files", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]string{"type": "string"}}}},
					{"name": "execute_command", "description": "Execute a shell command", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]string{"type": "string"}}}},
				},
			}, nil, req.ID)

		case "tools/call":
			if delay > 0 {
				time.Sleep(delay)
			}
			var params struct {
				Name string `json:"name"`
			}
			json.Unmarshal(req.Params, &params)
			writeJSONRPC(w, map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("executed %s successfully", params.Name)},
				},
			}, nil, req.ID)

		case "resources/list":
			writeJSONRPC(w, map[string]interface{}{
				"resources": []map[string]interface{}{
					{"uri": "file:///config.yaml", "name": "config.yaml", "mimeType": "text/yaml"},
					{"uri": "file:///readme.md", "name": "readme.md", "mimeType": "text/markdown"},
				},
			}, nil, req.ID)

		case "resources/read":
			writeJSONRPC(w, map[string]interface{}{
				"contents": []map[string]interface{}{
					{"uri": "file:///config.yaml", "text": "server:\n  listen: :8080\n", "mimeType": "text/yaml"},
				},
			}, nil, req.ID)

		case "prompts/list":
			writeJSONRPC(w, map[string]interface{}{
				"prompts": []map[string]interface{}{
					{"name": "code_review", "description": "Review code for issues"},
					{"name": "summarize", "description": "Summarize text"},
				},
			}, nil, req.ID)

		case "prompts/get":
			writeJSONRPC(w, map[string]interface{}{
				"description": "Review code",
				"messages": []map[string]interface{}{
					{"role": "user", "content": map[string]interface{}{"type": "text", "text": "Please review this code."}},
				},
			}, nil, req.ID)

		default:
			writeJSONRPC(w, nil, map[string]interface{}{"code": -32601, "message": fmt.Sprintf("Method not found: %s", req.Method)}, req.ID)
		}
	}))
}

func writeJSONRPC(w http.ResponseWriter, result interface{}, rpcErr interface{}, id interface{}) {
	resp := map[string]interface{}{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Pipeline builders ---

// newE2EPipeline creates the standard E2E pipeline with multi-token auth and configurable options.
func newE2EPipeline(t *testing.T, upstreamURL string, opts ...proxy.PipelineOption) (*httptest.Server, *memAuditLogger) {
	t.Helper()
	cfg := &config.Config{Server: config.ServerConfig{UpstreamURL: upstreamURL}}
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	roles := []config.RoleConfig{
		{Name: "admin", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "readonly", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "restricted", AllowedTools: []string{"read_file", "list_dir"}, DenyTools: []string{}},
	}
	rbacEngine := rbac.NewEngine(roles)

	rlCfg := &config.RateLimitConfig{RequestsPerMinute: 200, BurstSize: 20}
	rateLimiter := ratelimit.NewLimiter(rlCfg)

	auditLog := &memAuditLogger{}
	auth := &multiTokenAuth{}

	defaultOpts := []proxy.PipelineOption{proxy.WithMaxBodySize(1048576)} // 1MB
	allOpts := append(defaultOpts, opts...)

	pipeline := proxy.NewPipelineForTest(handler, auth, rateLimiter, rbacEngine, nil, auditLog, allOpts...)
	server := httptest.NewServer(pipeline)
	t.Cleanup(server.Close)
	return server, auditLog
}

// newFreeTierPipeline creates a pipeline with free-tier rate limits (10 RPM, burst 5).
func newFreeTierPipeline(t *testing.T, upstreamURL string) (*httptest.Server, *memAuditLogger) {
	t.Helper()
	cfg := &config.Config{Server: config.ServerConfig{UpstreamURL: upstreamURL}}
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	roles := []config.RoleConfig{
		{Name: "admin", AllowedTools: []string{}, DenyTools: []string{}},
		{Name: "readonly", AllowedTools: []string{}, DenyTools: []string{}},
	}
	rbacEngine := rbac.NewEngine(roles)

	rlCfg := &config.RateLimitConfig{RequestsPerMinute: 10, BurstSize: 5}
	rateLimiter := ratelimit.NewLimiter(rlCfg)

	auditLog := &memAuditLogger{}
	auth := &multiTokenAuth{}

	pipeline := proxy.NewPipelineForTest(handler, auth, rateLimiter, rbacEngine, nil, auditLog)
	server := httptest.NewServer(pipeline)
	t.Cleanup(server.Close)
	return server, auditLog
}

// --- JSON-RPC helpers ---

func jsonRPCBody(method string, params interface{}) []byte {
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "method": method, "params": params, "id": 1,
	})
	return body
}

func toolsCallBody(toolName string) []byte {
	return jsonRPCBody("tools/call", map[string]interface{}{
		"name": toolName, "arguments": map[string]interface{}{},
	})
}

func batchBody(items ...[]byte) []byte {
	var batch []json.RawMessage
	for _, item := range items {
		batch = append(batch, json.RawMessage(item))
	}
	out, _ := json.Marshal(batch)
	return out
}

func sendReq(t *testing.T, url string, body []byte, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
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

func sendReqWithHeaders(t *testing.T, url string, body []byte, token string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return body
}

func decodeJSON(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, string(data))
	}
	return result
}

// --- License key helpers ---

func generateTestLicenseKey(t *testing.T, tier string) (string, *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	claims := map[string]interface{}{
		"tier":          tier,
		"max_upstreams": 5,
		"max_rpm":       200,
		"iss":           "mcpzerotrust.dev",
		"sub":           "e2e-test",
		"iat":           time.Now().Unix(),
		"exp":           time.Now().Add(24 * time.Hour).Unix(),
	}
	if tier == "enterprise" {
		claims["max_upstreams"] = 0 // unlimited
		claims["max_rpm"] = 0       // unlimited
	}

	headerJSON, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT"})
	claimsJSON, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	sigInput := header + "." + payload

	sig, err := license.SignJWT(priv, sigInput)
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	return sigInput + "." + sig, priv
}

// --- Findings ---

// finding represents a single test observation for the report.
type finding struct {
	Persona  string `json:"persona"`
	Category string `json:"category"` // "pass", "fail", "gap", "warning"
	Summary  string `json:"summary"`
	Detail   string `json:"detail,omitempty"`
}

var (
	findingsMu sync.Mutex
	findings   []finding
)

func record(f finding) {
	findingsMu.Lock()
	defer findingsMu.Unlock()
	findings = append(findings, f)
}

func allFindings() []finding {
	findingsMu.Lock()
	defer findingsMu.Unlock()
	out := make([]finding, len(findings))
	copy(out, findings)
	return out
}

// TestE2E_HealthCheck is the Layer 1 smoke test -- verifies the proxy boots and responds.
func TestE2E_HealthCheck(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	server, _ := newE2EPipeline(t, mock.URL)

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var health map[string]string
	json.Unmarshal(body, &health)
	if health["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", health["status"])
	}
	t.Logf("Layer 1 smoke test PASSED: /health returned {\"status\":\"ok\"}")
}

// TestE2E_BasicPipeline verifies the full pipeline works with multi-token auth.
func TestE2E_BasicPipeline(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	server, auditLog := newE2EPipeline(t, mock.URL)

	// Test with Marcus token
	resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenMarcus)
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
	result := decodeJSON(t, body)
	if result["error"] != nil {
		t.Fatalf("unexpected error: %v", result["error"])
	}
	t.Logf("Marcus tools/list: OK")

	// Test with Priya intern token (readonly)
	resp2 := sendReq(t, server.URL, toolsCallBody("read_file"), tokenPriyaIntern)
	body2 := readBody(t, resp2)
	result2 := decodeJSON(t, body2)
	if result2["error"] == nil {
		t.Fatalf("expected readonly to be denied tools/call, but got result: %v", result2["result"])
	}
	t.Logf("Priya intern tools/call denied: OK")

	// Verify audit captured both
	entries := auditLog.snapshot()
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 audit entries, got %d", len(entries))
	}
	t.Logf("Audit captured %d entries: OK", len(entries))
	t.Logf("Layer 1 basic pipeline PASSED: multi-token auth + RBAC + audit working")
}
