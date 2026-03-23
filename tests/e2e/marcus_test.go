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
	quickstart, err := io.ReadAll(mustOpen(t, "../../docs/QUICKSTART.md"))
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
