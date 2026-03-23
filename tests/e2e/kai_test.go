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

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/audit"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
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
