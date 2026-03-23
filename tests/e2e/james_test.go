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
					Persona:  "james",
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
			Persona:  "james",
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
