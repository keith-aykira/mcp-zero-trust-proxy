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
