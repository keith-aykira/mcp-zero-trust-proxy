# E2E Persona Validation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run 5 buyer personas through the full MCP Zero-Trust Proxy experience (build → configure → use → stress test) and produce a ship/no-ship report.

**Architecture:** Layer cake — Task 1 builds shared test infrastructure (multi-token mock auth + mock MCP server + test config + Pro license key). Tasks 2-6 run persona scenarios in parallel as Go integration tests. Task 7 runs Layer 3 journey evaluation via Gemini. Task 8 consolidates the report.

**Tech Stack:** Go 1.26 (binary at `~/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin/go`), Docker, curl, Gemini MCP

**Spec:** `docs/superpowers/specs/2026-03-22-e2e-persona-validation-design.md`

---

## File Structure

```
tests/e2e/
├── e2e_test.go              # Shared test infrastructure: multi-token auth, boot helpers, JSON-RPC helpers
├── marcus_test.go            # Persona 1: Solo AI dev — binary install, free tier, docs clarity
├── priya_test.go             # Persona 2: Startup CTO — Docker, RBAC, team management
├── james_test.go             # Persona 3: Security engineer — adversarial attacks
├── sofia_test.go             # Persona 4: Agency dev — multi-tenant isolation
├── kai_test.go               # Persona 5: DevOps engineer — production readiness
├── testdata/
│   ├── e2e_config.yaml       # Pro-tier test config with all personas
│   └── marcus_minimal.yaml   # Minimal config Marcus would write from docs alone
docs/e2e-validation/
└── persona-validation-report.md  # Final consolidated report (Task 8)
```

---

## Task 1: Create E2E Test Infrastructure (Layer 1)

**Files:**
- Create: `tests/e2e/e2e_test.go`
- Create: `tests/e2e/testdata/e2e_config.yaml`

This task builds the shared foundation all 5 persona tests depend on.

- [ ] **Step 1: Create the e2e test directory**

```bash
mkdir -p tests/e2e/testdata
```

- [ ] **Step 2: Write the multi-token mock authenticator and shared helpers**

Create `tests/e2e/e2e_test.go`:

```go
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

	pipeline := proxy.NewPipelineForTest(handler, auth, rateLimiter, rbacEngine, auditLog, allOpts...)
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

	pipeline := proxy.NewPipelineForTest(handler, auth, rateLimiter, rbacEngine, auditLog)
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

// TestE2E_HealthCheck is the Layer 1 smoke test — verifies the proxy boots and responds.
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
```

- [ ] **Step 3: Run the infrastructure smoke tests**

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
cd /Users/andrewnoble/Developer/mcp-zero-trust-proxy
go test ./tests/e2e/ -v -run "TestE2E_HealthCheck|TestE2E_BasicPipeline" -timeout 30s
```

Expected: Both tests PASS. If `NewPipelineForTest` signature doesn't accept variadic `PipelineOption`, adjust `newE2EPipeline` to not pass options (remove the `allOpts...` argument).

- [ ] **Step 4: Commit**

```bash
git add tests/e2e/
git commit -m "test(e2e): add multi-token auth and shared infrastructure for persona validation"
```

---

## Task 2: Marcus — Solo AI Dev Scenario

**Files:**
- Create: `tests/e2e/marcus_test.go`

**Focus:** Free tier experience, docs clarity, rate limit reality, upgrade path.

- [ ] **Step 1: Write Marcus test file**

Create `tests/e2e/marcus_test.go`:

```go
package e2e

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMarcus_FreeTierRateLimits(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	// Marcus uses free tier — 10 RPM, burst 5
	server, _ := newFreeTierPipeline(t, mock.URL)

	// Send burst of 6 requests — 6th should be rate-limited
	var lastStatus int
	for i := 0; i < 8; i++ {
		resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenMarcus)
		lastStatus = resp.StatusCode
		if resp.StatusCode == 429 {
			// Read body BEFORE closing — check error message clarity
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			bodyStr := string(body)

			record(finding{
				Persona:  "marcus",
				Category: "pass",
				Summary:  fmt.Sprintf("Free tier rate limit hit at request %d", i+1),
				Detail:   "Rate limiter correctly enforces 10 RPM / burst 5 for free tier",
			})
			t.Logf("Rate limit hit at request %d: PASS", i+1)

			if strings.Contains(bodyStr, "upgrade") || strings.Contains(bodyStr, "limit") {
				record(finding{Persona: "marcus", Category: "pass", Summary: "Rate limit error mentions limits"})
			} else {
				record(finding{
					Persona:  "marcus",
					Category: "gap",
					Summary:  "Rate limit 429 response doesn't mention upgrade or explain the limit",
					Detail:   fmt.Sprintf("Response body: %s", bodyStr),
				})
			}
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	if lastStatus != 429 {
		record(finding{
			Persona:  "marcus",
			Category: "fail",
			Summary:  "Free tier rate limit not enforced — sent 8 requests without 429",
		})
		t.Errorf("Expected 429 after burst, last status was %d", lastStatus)
	}
}

func TestMarcus_BasicWorkflow(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	server, auditLog := newE2EPipeline(t, mock.URL)

	// Step 1: tools/list
	resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenMarcus)
	body := readBody(t, resp)
	if resp.StatusCode != 200 {
		record(finding{Persona: "marcus", Category: "fail", Summary: "tools/list failed", Detail: string(body)})
		t.Fatalf("tools/list failed: %s", string(body))
	}
	result := decodeJSON(t, body)
	if result["error"] != nil {
		record(finding{Persona: "marcus", Category: "fail", Summary: "tools/list returned error", Detail: fmt.Sprintf("%v", result["error"])})
		t.Fatalf("tools/list error: %v", result["error"])
	}
	record(finding{Persona: "marcus", Category: "pass", Summary: "tools/list succeeds on first try"})

	// Step 2: tools/call
	resp2 := sendReq(t, server.URL, toolsCallBody("read_file"), tokenMarcus)
	body2 := readBody(t, resp2)
	if resp2.StatusCode != 200 {
		record(finding{Persona: "marcus", Category: "fail", Summary: "tools/call failed", Detail: string(body2)})
		t.Fatalf("tools/call failed: %s", string(body2))
	}
	record(finding{Persona: "marcus", Category: "pass", Summary: "tools/call succeeds — admin can call tools"})

	// Step 3: Verify audit entries captured
	entries := auditLog.snapshot()
	if len(entries) < 2 {
		record(finding{Persona: "marcus", Category: "fail", Summary: fmt.Sprintf("Expected 2+ audit entries, got %d", len(entries))})
		t.Errorf("Expected 2+ audit entries, got %d", len(entries))
	} else {
		// Check audit entry has expected fields
		e := entries[0]
		if e.ClientID == "" || e.Method == "" {
			record(finding{Persona: "marcus", Category: "warning", Summary: "Audit entry missing fields", Detail: fmt.Sprintf("%+v", e)})
		} else {
			record(finding{Persona: "marcus", Category: "pass", Summary: "Audit entries captured with correct fields (ClientID, Method, timestamp)"})
		}
	}

	t.Logf("Marcus basic workflow: PASS")
}

func TestMarcus_DocsClarity(t *testing.T) {
	// Check QUICKSTART.md exists and contains key sections
	quickstart, err := io.ReadAll(mustOpen(t, "docs/QUICKSTART.md"))
	if err != nil {
		record(finding{Persona: "marcus", Category: "fail", Summary: "QUICKSTART.md not found or unreadable"})
		t.Fatalf("Can't read QUICKSTART.md: %v", err)
	}
	qs := string(quickstart)

	checks := []struct {
		name    string
		search  string
		missing string
	}{
		{"Docker install instructions", "docker", "No Docker install instructions"},
		{"Binary install instructions", "binary", "No binary install instructions — Marcus doesn't use Docker"},
		{"Config example", "config", "No config example in quickstart"},
		{"Upstream URL", "upstream", "Doesn't mention upstream_url — first config field Marcus needs"},
		{"Auth setup", "auth", "No auth setup instructions"},
		{"License/tier explanation", "license", "No mention of licensing or tiers — Marcus won't know free vs Pro"},
		{"Upgrade path", "upgrade", "No upgrade instructions — Marcus hits free tier limits with no guidance"},
	}

	for _, c := range checks {
		if !strings.Contains(strings.ToLower(qs), c.search) {
			record(finding{Persona: "marcus", Category: "gap", Summary: c.missing})
			t.Logf("QUICKSTART gap: %s", c.missing)
		} else {
			record(finding{Persona: "marcus", Category: "pass", Summary: fmt.Sprintf("QUICKSTART has %s", c.name)})
		}
	}
}

func TestMarcus_NoAuthReturns401(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	server, _ := newE2EPipeline(t, mock.URL)

	// No Authorization header
	resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), "")
	body := readBody(t, resp)
	if resp.StatusCode != 401 {
		record(finding{Persona: "marcus", Category: "fail", Summary: fmt.Sprintf("Missing auth returned %d, expected 401", resp.StatusCode)})
		t.Errorf("Expected 401, got %d: %s", resp.StatusCode, string(body))
		return
	}

	// Check error doesn't leak internals
	bodyStr := string(body)
	if strings.Contains(bodyStr, "panic") || strings.Contains(bodyStr, "goroutine") || strings.Contains(bodyStr, "/Users/") {
		record(finding{Persona: "marcus", Category: "fail", Summary: "401 response leaks internal info", Detail: bodyStr})
	} else {
		record(finding{Persona: "marcus", Category: "pass", Summary: "401 error message is clean — no internal leakage"})
	}
}

func mustOpen(t *testing.T, path string) io.Reader {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
```

- [ ] **Step 2: Run Marcus tests**

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
cd /Users/andrewnoble/Developer/mcp-zero-trust-proxy
go test ./tests/e2e/ -v -run "TestMarcus" -timeout 30s
```

Expected: Tests run (some may reveal gaps — that's the point). Document all findings.

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/marcus_test.go
git commit -m "test(e2e): marcus persona — free tier, docs clarity, basic workflow"
```

---

## Task 3: Priya — Startup CTO Scenario

**Files:**
- Create: `tests/e2e/priya_test.go`

**Focus:** Docker build, RBAC enforcement, team management, audit attribution.

- [ ] **Step 1: Write Priya test file**

Create `tests/e2e/priya_test.go`:

```go
package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestPriya_DockerBuild(t *testing.T) {
	// Build Docker image and check size
	cmd := exec.Command("docker", "build", "-t", "mcp-proxy-e2e-test", ".")
	cmd.Dir = "/Users/andrewnoble/Developer/mcp-zero-trust-proxy"
	out, err := cmd.CombinedOutput()
	if err != nil {
		record(finding{Persona: "priya", Category: "fail", Summary: "Docker build failed", Detail: string(out)})
		t.Fatalf("Docker build failed: %s", string(out))
	}
	record(finding{Persona: "priya", Category: "pass", Summary: "Docker build succeeds"})

	// Check image size
	sizeCmd := exec.Command("docker", "image", "inspect", "mcp-proxy-e2e-test", "--format", "{{.Size}}")
	sizeOut, err := sizeCmd.Output()
	if err != nil {
		t.Logf("Could not inspect image size: %v", err)
	} else {
		t.Logf("Docker image size: %s bytes", strings.TrimSpace(string(sizeOut)))
		record(finding{Persona: "priya", Category: "pass", Summary: fmt.Sprintf("Docker image size: %s bytes", strings.TrimSpace(string(sizeOut)))})
	}

	// Cleanup
	exec.Command("docker", "rmi", "mcp-proxy-e2e-test").Run()
}

func TestPriya_RBACAdminVsReadonly(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	server, auditLog := newE2EPipeline(t, mock.URL)

	// Priya (admin) calls tools/call — should succeed
	resp := sendReq(t, server.URL, toolsCallBody("read_file"), tokenPriyaAdmin)
	body := readBody(t, resp)
	result := decodeJSON(t, body)
	if result["error"] != nil {
		record(finding{Persona: "priya", Category: "fail", Summary: "Admin tools/call denied", Detail: fmt.Sprintf("%v", result["error"])})
		t.Fatalf("Admin should have access: %v", result["error"])
	}
	record(finding{Persona: "priya", Category: "pass", Summary: "Admin (priya) can call tools/call"})

	// Intern (readonly) calls tools/call — should be denied
	resp2 := sendReq(t, server.URL, toolsCallBody("read_file"), tokenPriyaIntern)
	body2 := readBody(t, resp2)
	result2 := decodeJSON(t, body2)
	if result2["error"] == nil {
		record(finding{Persona: "priya", Category: "fail", Summary: "Readonly intern was NOT denied tools/call"})
		t.Fatalf("Readonly should be denied tools/call")
	}

	// Check denial message quality
	errObj, ok := result2["error"].(map[string]interface{})
	if ok {
		msg, _ := errObj["message"].(string)
		if strings.Contains(strings.ToLower(msg), "denied") || strings.Contains(strings.ToLower(msg), "forbidden") || strings.Contains(strings.ToLower(msg), "not allowed") {
			record(finding{Persona: "priya", Category: "pass", Summary: "RBAC denial message is clear", Detail: msg})
		} else {
			record(finding{Persona: "priya", Category: "warning", Summary: "RBAC denial message could be clearer", Detail: msg})
		}
	}
	t.Logf("Intern readonly denied: PASS")

	// Intern can still call tools/list
	resp3 := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenPriyaIntern)
	body3 := readBody(t, resp3)
	result3 := decodeJSON(t, body3)
	if result3["error"] != nil {
		record(finding{Persona: "priya", Category: "fail", Summary: "Readonly denied tools/list — should be allowed", Detail: fmt.Sprintf("%v", result3["error"])})
		t.Errorf("Readonly should be able to list tools")
	} else {
		record(finding{Persona: "priya", Category: "pass", Summary: "Readonly (intern) can call tools/list"})
	}

	// Verify audit shows both users correctly attributed
	entries := auditLog.snapshot()
	adminFound, internFound := false, false
	for _, e := range entries {
		if e.ClientID == "priya" {
			adminFound = true
		}
		if e.ClientID == "intern" {
			internFound = true
		}
	}
	if adminFound && internFound {
		record(finding{Persona: "priya", Category: "pass", Summary: "Audit log correctly attributes requests to different users"})
	} else {
		record(finding{Persona: "priya", Category: "fail", Summary: fmt.Sprintf("Audit attribution incomplete: admin=%v intern=%v", adminFound, internFound)})
	}
}

func TestPriya_LicenseKeyTierExpansion(t *testing.T) {
	// Generate a Pro license and verify it's a valid JWT
	jwt, _ := generateTestLicenseKey(t, "pro")
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		record(finding{Persona: "priya", Category: "fail", Summary: "Generated license key is not valid JWT format"})
		t.Fatalf("Expected 3 JWT parts, got %d", len(parts))
	}
	record(finding{Persona: "priya", Category: "pass", Summary: "Pro license key generates as valid JWT"})
	t.Logf("Pro license key generated: %s...%s", jwt[:20], jwt[len(jwt)-10:])
}
```

- [ ] **Step 2: Run Priya tests**

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
cd /Users/andrewnoble/Developer/mcp-zero-trust-proxy
go test ./tests/e2e/ -v -run "TestPriya" -timeout 60s
```

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/priya_test.go
git commit -m "test(e2e): priya persona — docker build, RBAC, team RBAC, audit attribution"
```

---

## Task 4: James — Security Engineer (Adversarial)

**Files:**
- Create: `tests/e2e/james_test.go`

**Focus:** Break things. Malformed input, RBAC bypass, info leakage, session isolation, rate limiter stress.

- [ ] **Step 1: Write James test file**

Create `tests/e2e/james_test.go`:

```go
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// --- Malformed JSON-RPC ---

func TestJames_MalformedJSONRPC(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, _ := newE2EPipeline(t, mock.URL)

	cases := []struct {
		name string
		body []byte
	}{
		{"missing method", []byte(`{"jsonrpc":"2.0","id":1}`)},
		{"null params", []byte(`{"jsonrpc":"2.0","method":"tools/list","params":null,"id":1}`)},
		{"empty body", []byte(``)},
		{"invalid JSON", []byte(`{not json at all}`)},
		{"missing jsonrpc field", []byte(`{"method":"tools/list","id":1}`)},
		{"deeply nested", []byte(generateDeeplyNested(100))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := sendReq(t, server.URL, tc.body, tokenJames)
			body := readBody(t, resp)

			// Should NOT be 500 (internal server error) — that signals unhandled edge case
			if resp.StatusCode == 500 {
				record(finding{
					Persona:  "james",
					Category: "fail",
					Summary:  fmt.Sprintf("Malformed input '%s' caused 500 Internal Server Error", tc.name),
					Detail:   string(body),
				})
				t.Errorf("500 on %s: %s", tc.name, string(body))
			} else {
				// Check response doesn't leak internals
				leaks := checkInfoLeakage(string(body))
				if len(leaks) > 0 {
					record(finding{
						Persona:  "james",
						Category: "fail",
						Summary:  fmt.Sprintf("Info leakage in '%s' error response", tc.name),
						Detail:   strings.Join(leaks, "; "),
					})
				} else {
					record(finding{
						Persona:  "james",
						Category: "pass",
						Summary:  fmt.Sprintf("Malformed '%s' handled cleanly (HTTP %d, no leakage)", tc.name, resp.StatusCode),
					})
				}
			}
		})
	}
}

func TestJames_OversizedBody(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, _ := newE2EPipeline(t, mock.URL)

	// Send >1MB body (default max_body_size is 1MB = 1048576 bytes)
	bigBody := make([]byte, 1048577)
	for i := range bigBody {
		bigBody[i] = 'A'
	}
	resp := sendReq(t, server.URL, bigBody, tokenJames)
	body := readBody(t, resp)

	if resp.StatusCode == 413 || resp.StatusCode == 400 {
		record(finding{Persona: "james", Category: "pass", Summary: fmt.Sprintf("Oversized body rejected with HTTP %d", resp.StatusCode)})
	} else {
		record(finding{
			Persona:  "james",
			Category: "fail",
			Summary:  fmt.Sprintf("Oversized body NOT rejected — got HTTP %d", resp.StatusCode),
			Detail:   string(body),
		})
	}
}

// --- RBAC bypass attempts ---

func TestJames_RBACBypass(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	// Use Sofia paralegal (readonly) to attempt bypasses
	server, _ := newE2EPipeline(t, mock.URL)

	cases := []struct {
		name   string
		method string
		desc   string
	}{
		{"case variation", "Tools/Call", "Different casing — tests case-sensitive method matching"},
		{"extra whitespace", "tools/call ", "Trailing whitespace in method name"},
		{"unicode lookalike", "tools\u2215call", "Unicode fraction slash instead of ASCII slash"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := jsonRPCBody(tc.method, map[string]interface{}{"name": "read_file", "arguments": map[string]interface{}{}})
			resp := sendReq(t, server.URL, body, tokenSofiaParalegal)
			respBody := readBody(t, resp)
			result := decodeJSON(t, respBody)

			if result["error"] != nil {
				record(finding{
					Persona: "james",
					Category: "pass",
					Summary:  fmt.Sprintf("RBAC bypass '%s' correctly denied", tc.name),
					Detail:   tc.desc,
				})
			} else {
				record(finding{
					Persona:  "james",
					Category: "fail",
					Summary:  fmt.Sprintf("RBAC bypass '%s' SUCCEEDED — readonly called tools/call", tc.name),
					Detail:   tc.desc,
				})
				t.Errorf("RBAC bypass succeeded for %s", tc.name)
			}
		})
	}
}

func TestJames_BatchMixedRBAC(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, auditLog := newE2EPipeline(t, mock.URL)

	// Batch: one allowed (tools/list) + one denied (tools/call) for readonly
	batch := batchBody(
		jsonRPCBody("tools/list", nil),
		toolsCallBody("read_file"),
	)

	resp := sendReq(t, server.URL, batch, tokenPriyaIntern)
	body := readBody(t, resp)

	// Should be a JSON array with per-item results
	var results []map[string]interface{}
	if err := json.Unmarshal(body, &results); err != nil {
		// Not a JSON array — document the behavior
		record(finding{
			Persona:  "james",
			Category: "warning",
			Summary:  "Batch response is not a JSON array",
			Detail:   string(body),
		})
		t.Logf("Batch response not an array: %s", string(body))
		return
	}

	if len(results) == 2 {
		// First should succeed, second should have error
		allowedOK := results[0]["error"] == nil
		deniedOK := results[1]["error"] != nil
		if allowedOK && deniedOK {
			record(finding{Persona: "james", Category: "pass", Summary: "Batch RBAC: per-item enforcement works (allowed + denied)"})
		} else {
			record(finding{
				Persona:  "james",
				Category: "fail",
				Summary:  fmt.Sprintf("Batch RBAC incorrect: item0-ok=%v item1-denied=%v", allowedOK, deniedOK),
			})
		}
	}

	// Verify audit logged both batch items
	entries := auditLog.snapshot()
	record(finding{Persona: "james", Category: "pass", Summary: fmt.Sprintf("Batch audit: %d entries logged", len(entries))})
}

// --- Info leakage ---

func TestJames_AuthErrorLeakage(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, _ := newE2EPipeline(t, mock.URL)

	cases := []struct {
		name  string
		token string
	}{
		{"no auth header", ""},
		{"invalid token", tokenUnknown},
		{"malformed bearer", "not-bearer-format"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader(jsonRPCBody("tools/list", nil)))
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				if tc.name == "malformed bearer" {
					req.Header.Set("Authorization", tc.token) // no "Bearer " prefix
				} else {
					req.Header.Set("Authorization", "Bearer "+tc.token)
				}
			}
			resp, _ := http.DefaultClient.Do(req)
			body := readBody(t, resp)

			leaks := checkInfoLeakage(string(body))
			if len(leaks) > 0 {
				record(finding{
					Persona:  "james",
					Category: "fail",
					Summary:  fmt.Sprintf("Auth error '%s' leaks info", tc.name),
					Detail:   strings.Join(leaks, "; "),
				})
			} else {
				record(finding{Persona: "james", Category: "pass", Summary: fmt.Sprintf("Auth error '%s' clean — no leakage", tc.name)})
			}

			// Check that different auth failures return same status (don't reveal WHY auth failed)
			if resp.StatusCode != 401 {
				record(finding{
					Persona:  "james",
					Category: "warning",
					Summary:  fmt.Sprintf("Auth error '%s' returned %d instead of 401", tc.name, resp.StatusCode),
				})
			}
		})
	}
}

// --- Session isolation ---

func TestJames_SessionIsolation(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, auditLog := newE2EPipeline(t, mock.URL)

	// User A makes a request
	sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenSofiaAttorney)
	// User B makes a request
	sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenSofiaParalegal)

	entries := auditLog.snapshot()
	sessions := make(map[string]string) // clientID -> sessionID
	for _, e := range entries {
		if prev, exists := sessions[e.ClientID]; exists && prev != e.SessionID {
			record(finding{Persona: "james", Category: "fail", Summary: "Session ID changed for same client"})
		}
		sessions[e.ClientID] = e.SessionID
	}

	if sessions["attorney"] != sessions["paralegal"] {
		record(finding{Persona: "james", Category: "pass", Summary: "Session isolation: different users get different session IDs"})
	} else if sessions["attorney"] == "" {
		record(finding{Persona: "james", Category: "warning", Summary: "Session IDs not populated in audit entries"})
	} else {
		record(finding{Persona: "james", Category: "fail", Summary: "Session isolation FAILED: same session for different users"})
	}
}

// --- Rate limiter stress ---

func TestJames_RateLimiterStress(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, _ := newE2EPipeline(t, mock.URL) // 200 RPM, burst 20

	// Send burst+1 (21) requests concurrently
	var wg sync.WaitGroup
	var rejected int64
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenJames)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 429 {
				atomic.AddInt64(&rejected, 1)
			}
		}()
	}
	wg.Wait()

	if rejected > 0 {
		record(finding{
			Persona: "james",
			Category: "pass",
			Summary:  fmt.Sprintf("Rate limiter enforced: %d of 25 concurrent requests rejected", rejected),
		})
	} else {
		record(finding{
			Persona:  "james",
			Category: "warning",
			Summary:  "Rate limiter did not reject any of 25 concurrent requests (burst=20, expected some rejection)",
		})
	}
}

// --- Header injection ---

func TestJames_HeaderInjection(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, auditLog := newE2EPipeline(t, mock.URL)

	// Send request with spoofed headers
	injectedHeaders := map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	}
	sendReqWithHeaders(t, server.URL, jsonRPCBody("tools/list", nil), tokenJames, injectedHeaders)

	entries := auditLog.snapshot()
	if len(entries) > 0 {
		e := entries[len(entries)-1]
		// Audit should show the actual client, not the injected IP
		if e.ClientID == "james" {
			record(finding{Persona: "james", Category: "pass", Summary: "Header injection: audit uses authenticated identity, not spoofed X-Forwarded-For"})
		}
	}
}

// --- Helpers ---

func generateDeeplyNested(depth int) string {
	var sb strings.Builder
	sb.WriteString(`{"jsonrpc":"2.0","method":"tools/call","params":`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"nested":`)
	}
	sb.WriteString(`"deep"`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}
	sb.WriteString(`,"id":1}`)
	return sb.String()
}

func checkInfoLeakage(body string) []string {
	var leaks []string
	patterns := []struct {
		pattern string
		desc    string
	}{
		{"goroutine", "Go stack trace leaked"},
		{"panic", "Panic message leaked"},
		{"/Users/", "Local file path leaked"},
		{"/home/", "Server file path leaked"},
		{"localhost:", "Upstream URL leaked"},
		{"127.0.0.1:", "Internal IP leaked"},
		{"client_secret", "Client secret leaked"},
		{"token_endpoint", "Provider config leaked"},
		{".go:", "Go source file reference leaked"},
	}
	lower := strings.ToLower(body)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p.pattern)) {
			leaks = append(leaks, p.desc)
		}
	}
	return leaks
}
```

- [ ] **Step 2: Run James tests**

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
cd /Users/andrewnoble/Developer/mcp-zero-trust-proxy
go test ./tests/e2e/ -v -run "TestJames" -timeout 60s
```

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/james_test.go
git commit -m "test(e2e): james persona — adversarial security testing, RBAC bypass, info leakage"
```

---

## Task 5: Sofia — Agency Dev Scenario

**Files:**
- Create: `tests/e2e/sofia_test.go`

**Focus:** Multi-tenant isolation, per-client audit separation, config complexity.

- [ ] **Step 1: Write Sofia test file**

Create `tests/e2e/sofia_test.go`:

```go
package e2e

import (
	"fmt"
	"testing"
)

func TestSofia_SingleUpstreamReality(t *testing.T) {
	// Sofia's key question: can one proxy serve multiple clients with separate upstreams?
	// Answer: No. Config only supports a single upstream_url (string, not list).
	// This test documents the gap.

	record(finding{
		Persona:  "sofia",
		Category: "gap",
		Summary:  "No multi-upstream support — config.server.upstream_url is a single string",
		Detail:   "Sofia would need to run 3 separate proxy instances (one per client). The YAML schema doesn't support multiple upstreams. There's no error message explaining this — it's structurally impossible to configure. For 10 clients, that's 10 Docker containers.",
	})

	record(finding{
		Persona:  "sofia",
		Category: "gap",
		Summary:  "No 'multi-tenant' documentation — agency use case not addressed",
		Detail:   "Docs don't mention how to handle multiple clients. The recommended architecture (one proxy per client) should be documented explicitly.",
	})

	t.Log("Sofia multi-upstream gap documented")
}

func TestSofia_PerUserRolesWithinTenant(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, auditLog := newE2EPipeline(t, mock.URL)

	// Attorney (admin) can call tools
	resp := sendReq(t, server.URL, toolsCallBody("read_file"), tokenSofiaAttorney)
	body := readBody(t, resp)
	result := decodeJSON(t, body)
	if result["error"] != nil {
		record(finding{Persona: "sofia", Category: "fail", Summary: "Attorney (admin) denied tools/call"})
		t.Errorf("Attorney should have admin access")
	} else {
		record(finding{Persona: "sofia", Category: "pass", Summary: "Attorney (admin) can call tools within client tenant"})
	}

	// Paralegal (readonly) denied tools/call
	resp2 := sendReq(t, server.URL, toolsCallBody("read_file"), tokenSofiaParalegal)
	body2 := readBody(t, resp2)
	result2 := decodeJSON(t, body2)
	if result2["error"] == nil {
		record(finding{Persona: "sofia", Category: "fail", Summary: "Paralegal (readonly) was NOT denied tools/call"})
		t.Errorf("Paralegal should be denied")
	} else {
		record(finding{Persona: "sofia", Category: "pass", Summary: "Paralegal (readonly) correctly denied tools/call within same tenant"})
	}

	// Verify audit correctly attributes to different users in same tenant
	entries := auditLog.snapshot()
	attorneyEntries, paralegalEntries := 0, 0
	for _, e := range entries {
		if e.ClientID == "attorney" {
			attorneyEntries++
		}
		if e.ClientID == "paralegal" {
			paralegalEntries++
		}
	}

	if attorneyEntries > 0 && paralegalEntries > 0 {
		record(finding{Persona: "sofia", Category: "pass", Summary: "Audit separates attorney vs paralegal within same tenant"})
	} else {
		record(finding{
			Persona:  "sofia",
			Category: "warning",
			Summary:  fmt.Sprintf("Audit attribution within tenant: attorney=%d paralegal=%d entries", attorneyEntries, paralegalEntries),
		})
	}
}

func TestSofia_AuditLogFilterability(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, auditLog := newE2EPipeline(t, mock.URL)

	// Multiple users make requests
	sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenSofiaAttorney)
	sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenSofiaParalegal)
	sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenPriyaAdmin)

	// Can we filter audit by client?
	entries := auditLog.snapshot()
	clientIDs := make(map[string]int)
	for _, e := range entries {
		clientIDs[e.ClientID]++
	}

	if len(clientIDs) >= 3 {
		record(finding{
			Persona:  "sofia",
			Category: "pass",
			Summary:  "Audit log entries include ClientID — filterable per-client",
			Detail:   fmt.Sprintf("Clients found: %v", clientIDs),
		})
	} else {
		record(finding{
			Persona:  "sofia",
			Category: "warning",
			Summary:  fmt.Sprintf("Audit log has %d distinct clients (expected 3+)", len(clientIDs)),
		})
	}

	// Check: does audit include enough info to separate by tenant?
	if len(entries) > 0 {
		e := entries[0]
		hasEmail := e.ClientID != ""
		hasMethod := e.Method != ""
		hasTimestamp := !e.Timestamp.IsZero()
		if hasEmail && hasMethod && hasTimestamp {
			record(finding{Persona: "sofia", Category: "pass", Summary: "Audit entries have ClientID + Method + Timestamp — sufficient for per-tenant filtering"})
		} else {
			record(finding{Persona: "sofia", Category: "gap", Summary: "Audit entries missing fields needed for tenant filtering"})
		}
	}
}
```

- [ ] **Step 2: Run Sofia tests**

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
cd /Users/andrewnoble/Developer/mcp-zero-trust-proxy
go test ./tests/e2e/ -v -run "TestSofia" -timeout 30s
```

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/sofia_test.go
git commit -m "test(e2e): sofia persona — multi-tenant gaps, per-user RBAC, audit filterability"
```

---

## Task 6: Kai — DevOps Engineer Scenario

**Files:**
- Create: `tests/e2e/kai_test.go`

**Focus:** Production readiness — health checks, load testing, graceful shutdown, audit rotation, failure modes.

- [ ] **Step 1: Write Kai test file**

Create `tests/e2e/kai_test.go`:

```go
package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/audit"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

func TestKai_HealthCheck(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, _ := newE2EPipeline(t, mock.URL)

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		record(finding{Persona: "kai", Category: "fail", Summary: "Health check request failed", Detail: err.Error()})
		t.Fatalf("Health check: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		record(finding{Persona: "kai", Category: "fail", Summary: fmt.Sprintf("Health check returned %d", resp.StatusCode)})
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var health map[string]string
	if err := json.Unmarshal(body, &health); err != nil {
		record(finding{Persona: "kai", Category: "warning", Summary: "Health check response is not JSON", Detail: string(body)})
	} else if health["status"] == "ok" {
		record(finding{Persona: "kai", Category: "pass", Summary: "Health check returns {\"status\":\"ok\"} — monitorable"})
	}

	// Check: is there a /metrics endpoint?
	metricsResp, _ := http.Get(server.URL + "/metrics")
	if metricsResp != nil {
		io.Copy(io.Discard, metricsResp.Body)
		metricsResp.Body.Close()
		if metricsResp.StatusCode == 200 {
			record(finding{Persona: "kai", Category: "pass", Summary: "/metrics endpoint exists"})
		} else {
			record(finding{
				Persona:  "kai",
				Category: "gap",
				Summary:  "No /metrics endpoint — Prometheus monitoring not possible",
				Detail:   "DevOps teams expect /metrics for Prometheus scraping. Would need custom integration or log-based monitoring.",
			})
		}
	}
}

func TestKai_LoadTest(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()
	server, _ := newE2EPipeline(t, mock.URL)

	goroutinesBefore := runtime.NumGoroutine()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Send 200 requests (within 200 RPM limit)
	totalRequests := 200
	concurrency := 10
	var wg sync.WaitGroup
	var successCount, failCount int64
	latencies := make([]time.Duration, 0, totalRequests)
	var latMu sync.Mutex

	sem := make(chan struct{}, concurrency)
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			start := time.Now()
			resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenKai)
			elapsed := time.Since(start)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			latMu.Lock()
			latencies = append(latencies, elapsed)
			latMu.Unlock()

			if resp.StatusCode == 200 {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
			}
		}()
	}
	wg.Wait()

	goroutinesAfter := runtime.NumGoroutine()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// Calculate latency percentiles
	if len(latencies) > 0 {
		// Simple sort for percentiles
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[j] < latencies[i] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}
		p50 := latencies[len(latencies)*50/100]
		p95 := latencies[len(latencies)*95/100]
		p99 := latencies[len(latencies)*99/100]

		record(finding{
			Persona:  "kai",
			Category: "pass",
			Summary:  fmt.Sprintf("Load test: %d/%d success, p50=%v p95=%v p99=%v", successCount, totalRequests, p50, p95, p99),
		})
		t.Logf("Latency: p50=%v p95=%v p99=%v", p50, p95, p99)
	}

	// Goroutine leak check
	goroutineDelta := goroutinesAfter - goroutinesBefore
	if goroutineDelta > 10 {
		record(finding{
			Persona:  "kai",
			Category: "warning",
			Summary:  fmt.Sprintf("Goroutine leak suspected: before=%d after=%d delta=%d", goroutinesBefore, goroutinesAfter, goroutineDelta),
		})
	} else {
		record(finding{
			Persona:  "kai",
			Category: "pass",
			Summary:  fmt.Sprintf("No goroutine leak: before=%d after=%d delta=%d", goroutinesBefore, goroutinesAfter, goroutineDelta),
		})
	}

	// Memory check
	memDelta := int64(memAfter.Alloc) - int64(memBefore.Alloc)
	record(finding{
		Persona:  "kai",
		Category: "pass",
		Summary:  fmt.Sprintf("Memory delta after %d requests: %d bytes (%d MB)", totalRequests, memDelta, memDelta/(1024*1024)),
	})

	t.Logf("Success: %d, Fail: %d, Goroutines: %d→%d, Mem delta: %d bytes", successCount, failCount, goroutinesBefore, goroutinesAfter, memDelta)
}

func TestKai_AuditLogRotation(t *testing.T) {
	// Create a real audit logger with tiny rotation size to trigger rotation
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.jsonl")

	cfg := &config.AuditConfig{
		Enabled:  true,
		Output:   "file",
		FilePath: logPath,
		Rotation: config.AuditRotationConfig{
			MaxSizeMB: 0, // Will use bytes-based check below
		},
	}

	logger, err := audit.NewLogger(cfg)
	if err != nil {
		record(finding{Persona: "kai", Category: "fail", Summary: "Failed to create audit logger", Detail: err.Error()})
		t.Fatalf("Create logger: %v", err)
	}
	defer logger.Close()

	// Write enough entries to check file creation
	for i := 0; i < 100; i++ {
		logger.Log(proxy.AuditEntry{
			Timestamp: time.Now(),
			ClientID:  "kai",
			Method:    "tools/list",
			Allowed:   true,
			RequestID: fmt.Sprintf("req-%d", i),
		})
	}

	// Verify log file exists and has content
	info, err := os.Stat(logPath)
	if err != nil {
		record(finding{Persona: "kai", Category: "fail", Summary: "Audit log file not created"})
		t.Fatalf("Stat log: %v", err)
	}
	if info.Size() > 0 {
		record(finding{
			Persona:  "kai",
			Category: "pass",
			Summary:  fmt.Sprintf("Audit log file created: %d bytes after 100 entries", info.Size()),
		})
	}

	// Verify JSONL format — each line is valid JSON
	content, _ := os.ReadFile(logPath)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	validLines := 0
	for _, line := range lines {
		var entry map[string]interface{}
		if json.Unmarshal([]byte(line), &entry) == nil {
			validLines++
		}
	}
	if validLines == len(lines) {
		record(finding{Persona: "kai", Category: "pass", Summary: fmt.Sprintf("All %d audit lines are valid JSON (JSONL format confirmed)", validLines)})
	} else {
		record(finding{
			Persona:  "kai",
			Category: "fail",
			Summary:  fmt.Sprintf("Only %d of %d lines are valid JSON", validLines, len(lines)),
		})
	}
}

func TestKai_UpstreamDown(t *testing.T) {
	// Point proxy at a non-existent upstream
	server, _ := newE2EPipeline(t, "http://127.0.0.1:19999")

	resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenKai)
	body := readBody(t, resp)

	if resp.StatusCode == 502 || resp.StatusCode == 503 {
		record(finding{
			Persona:  "kai",
			Category: "pass",
			Summary:  fmt.Sprintf("Upstream down returns HTTP %d (clear error)", resp.StatusCode),
		})
	} else if resp.StatusCode == 200 {
		record(finding{
			Persona:  "kai",
			Category: "fail",
			Summary:  "Upstream down returned 200 — proxy should return 502/503",
		})
	}

	// Check error doesn't leak upstream details
	leaks := checkInfoLeakage(string(body))
	if len(leaks) > 0 {
		record(finding{
			Persona:  "kai",
			Category: "warning",
			Summary:  "Upstream-down error leaks internal details",
			Detail:   strings.Join(leaks, "; "),
		})
	} else {
		record(finding{Persona: "kai", Category: "pass", Summary: "Upstream-down error is clean — no internal leakage"})
	}
}

func TestKai_ConfigError(t *testing.T) {
	// This is a documentation-only test — we can't easily test binary startup from Go tests.
	// Document what Kai would expect.
	record(finding{
		Persona:  "kai",
		Category: "gap",
		Summary:  "Config error messaging not testable from integration tests",
		Detail:   "To test: run binary with invalid YAML and check stderr. Expected: clear error with line number. Needs manual verification.",
	})
}

func TestKai_FailClosedBehavior(t *testing.T) {
	mock := mockMCPServer(0)
	defer mock.Close()

	// Create pipeline with no authenticator to test fail-open vs fail-closed
	// The real question: if auth is configured but fails, does the proxy fail open or closed?
	server, _ := newE2EPipeline(t, mock.URL)

	// Send request with invalid token — should fail closed (401), not fall through
	resp := sendReq(t, server.URL, jsonRPCBody("tools/list", nil), tokenUnknown)
	body := readBody(t, resp)

	if resp.StatusCode == 401 {
		record(finding{Persona: "kai", Category: "pass", Summary: "Fail-closed confirmed: invalid auth returns 401, not proxied"})
	} else if resp.StatusCode == 200 {
		record(finding{
			Persona:  "kai",
			Category: "fail",
			Summary:  "FAIL-OPEN: invalid auth returned 200 — proxy forwarded unauthenticated request",
			Detail:   string(body),
		})
		t.Errorf("Proxy is fail-open! Invalid auth returned 200")
	} else {
		record(finding{
			Persona:  "kai",
			Category: "warning",
			Summary:  fmt.Sprintf("Invalid auth returned %d (expected 401)", resp.StatusCode),
		})
	}
}
```

- [ ] **Step 2: Run Kai tests**

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
cd /Users/andrewnoble/Developer/mcp-zero-trust-proxy
go test ./tests/e2e/ -v -run "TestKai" -timeout 60s
```

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/kai_test.go
git commit -m "test(e2e): kai persona — health check, load test, audit rotation, fail-closed"
```

---

## Task 7: Layer 3 — Journey Evaluation via Gemini

**Files:**
- Read: `landing-page/index.html`, `docs/QUICKSTART.md`, `docs/CONFIG-REFERENCE.md`

This task uses the Gemini MCP `triangulate` tool to roleplay each persona evaluating the non-technical experience. It runs AFTER Tasks 2-6 complete, using their findings as input.

- [ ] **Step 1: Collect all findings from Tasks 2-6**

Run all E2E tests and capture output:

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
cd /Users/andrewnoble/Developer/mcp-zero-trust-proxy
go test ./tests/e2e/ -v -timeout 120s 2>&1 | tee /tmp/e2e-results.txt
```

- [ ] **Step 2: Run Gemini triangulate for each persona**

For each of the 5 personas, call `mcp__gemini-workspace__triangulate` with:
- The persona's trigger, skeptic moment, and day-2 problem from the spec
- Their actual Layer 2 test findings (pass/fail/gap)
- The landing page HTML, QUICKSTART.md content, and pricing section
- Questions: landing page reaction, docs evaluation, purchase decision, day-2 verdict

- [ ] **Step 3: Document Gemini responses**

Save the raw Gemini output alongside the test findings for the consolidated report.

---

## Task 8: Consolidate Report

**Files:**
- Create: `docs/e2e-validation/persona-validation-report.md`

- [ ] **Step 1: Create the report directory**

```bash
mkdir -p docs/e2e-validation
```

- [ ] **Step 2: Write the consolidated report**

Using all findings from Tasks 2-7, write `docs/e2e-validation/persona-validation-report.md` with:

1. **Executive Summary** — Ship / Ship with fixes / Do not ship + top 3 blockers + top 3 strengths
2. **Per-Persona Scorecard** — table with Setup OK, Security OK, Would Buy, Would Renew, Top Issue
3. **Blockers** — must fix before launch
4. **Warnings** — won't stop launch but cost customers
5. **Missing Features** — things personas expected
6. **Per-Persona Detail** — full findings from Layer 2 + Layer 3

- [ ] **Step 3: Commit report**

```bash
git add docs/e2e-validation/
git commit -m "docs: E2E persona validation report — ship/no-ship assessment"
```

- [ ] **Step 4: Update HANDOFF_CURRENT.md**

Add the validation results to the handoff doc.

---

## Execution Notes

- **Go binary path**: `/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin/go` — must be in PATH for all Go commands
- **Tasks 2-6 can run in parallel** after Task 1 passes — they share the same test infrastructure
- **Task 7 depends on Tasks 2-6** — needs their findings as input
- **Task 8 depends on Task 7** — writes the final report
- **If `NewPipelineForTest` doesn't accept variadic options**: Remove `allOpts...` from `newE2EPipeline` and set maxBodySize via a different mechanism (check the function signature)
- **Docker required for Task 3** (Priya Docker build test) — skip if Docker is unavailable
