package audit_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/audit"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/middleware"
)

// makeEntry creates a test audit entry with all fields populated.
func makeEntry() middleware.AuditEntry {
	return middleware.AuditEntry{
		Timestamp:    time.Date(2026, 3, 19, 14, 30, 0, 0, time.UTC),
		ClientID:     "user123",
		SessionID:    "abc-def",
		Method:       "tools/call",
		ToolName:     "read_file",
		Allowed:      true,
		DeniedReason: "",
		Latency:      45 * time.Millisecond,
		RequestID:    "req-xyz",
	}
}

// newTestLogger creates a logger backed by a bytes.Buffer (for stdout) instead
// of the real os.Stdout, allowing us to inspect output in tests.
func newTestLogger(t *testing.T, cfg *config.AuditConfig, buf *bytes.Buffer) *audit.Logger {
	t.Helper()
	logger, err := audit.NewLoggerWithWriter(cfg, buf)
	if err != nil {
		t.Fatalf("NewLoggerWithWriter: %v", err)
	}
	return logger
}

// TestLog_ProducesValidJSON checks that a log entry writes parseable JSON.
func TestLog_ProducesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)

	logger.Log(makeEntry())

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("log output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
}

// TestLog_RequiredFields verifies all required fields appear in output.
func TestLog_RequiredFields(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)
	entry := makeEntry()
	logger.Log(entry)

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}

	required := []string{"timestamp", "client_id", "session_id", "method", "tool_name", "allowed", "denied_reason", "latency_ms", "request_id"}
	for _, field := range required {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field %q in log output", field)
		}
	}
}

// TestLog_StdoutJSONL verifies one JSON object per line.
func TestLog_StdoutJSONL(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)

	logger.Log(makeEntry())
	logger.Log(makeEntry())

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines of JSONL, got %d:\n%s", len(lines), buf.String())
	}
	for i, line := range lines {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("line %d is not valid JSON: %v\n%s", i+1, err, line)
		}
	}
}

// TestLog_FileOutput verifies that file output appends (does not overwrite).
func TestLog_FileOutput(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "audit.log")

	// Write first entry.
	cfg := &config.AuditConfig{Enabled: true, Output: "file", FilePath: filePath}
	logger1, err := audit.NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	logger1.Log(makeEntry())
	logger1.Close()

	// Write second entry with a new logger (simulates restart — should append).
	logger2, err := audit.NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger (second): %v", err)
	}
	logger2.Log(makeEntry())
	logger2.Close()

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (append mode), got %d:\n%s", len(lines), string(data))
	}
}

// TestLog_BothOutput verifies "both" mode writes to stdout AND file.
func TestLog_BothOutput(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "audit-both.log")

	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "both", FilePath: filePath}
	logger, err := audit.NewLoggerWithWriterAndFile(cfg, &buf, filePath)
	if err != nil {
		t.Fatalf("NewLoggerWithWriterAndFile: %v", err)
	}
	logger.Log(makeEntry())
	logger.Close()

	// Check stdout buffer has content.
	if buf.Len() == 0 {
		t.Error("expected stdout buffer to have output in 'both' mode")
	}

	// Check file has content.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected file to have output in 'both' mode")
	}
}

// TestLog_DeniedReason verifies denied_reason appears for denied requests.
func TestLog_DeniedReason(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)

	entry := makeEntry()
	entry.Allowed = false
	entry.DeniedReason = "role readonly cannot execute tools"
	logger.Log(entry)

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if result["denied_reason"] != "role readonly cannot execute tools" {
		t.Errorf("expected denied_reason to be set, got %v", result["denied_reason"])
	}
	if result["allowed"] != false {
		t.Errorf("expected allowed=false, got %v", result["allowed"])
	}
}

// TestLog_ISO8601Timestamp verifies timestamps are in ISO 8601 (RFC3339) UTC format.
func TestLog_ISO8601Timestamp(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)
	logger.Log(makeEntry())

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	ts, ok := result["timestamp"].(string)
	if !ok {
		t.Fatal("timestamp is not a string")
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try RFC3339Nano as fallback
		parsed, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			t.Fatalf("timestamp %q is not ISO 8601 / RFC3339: %v", ts, err)
		}
	}
	if parsed.Location() != time.UTC && parsed.UTC().Format(time.RFC3339) != ts[:len(time.RFC3339)-len("Z")+1] {
		// Just verify it's parseable and UTC
		if parsed.UTC().Hour() != parsed.Hour() {
			t.Errorf("timestamp %q is not UTC", ts)
		}
	}
}

// TestLog_ConcurrentSafe verifies concurrent logging does not interleave JSON lines.
func TestLog_ConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)

	const goroutines = 20
	const entriesEach = 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < entriesEach; j++ {
				logger.Log(makeEntry())
			}
		}()
	}
	wg.Wait()

	// All lines should be valid JSON (no interleaving).
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != goroutines*entriesEach {
		t.Errorf("expected %d lines, got %d", goroutines*entriesEach, len(lines))
	}
	for i, line := range lines {
		if line == "" {
			continue
		}
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("concurrent: line %d is not valid JSON (interleaved?): %v\n%s", i+1, err, line)
		}
	}
}

// TestLog_DisabledIsNoOp verifies that a disabled logger writes nothing.
func TestLog_DisabledIsNoOp(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: false, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)
	logger.Log(makeEntry())

	if buf.Len() != 0 {
		t.Errorf("expected no output from disabled logger, got %d bytes: %s", buf.Len(), buf.String())
	}
}

// TestLog_LatencyMs verifies latency is written as integer milliseconds.
func TestLog_LatencyMs(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.AuditConfig{Enabled: true, Output: "stdout"}
	logger := newTestLogger(t, cfg, &buf)

	entry := makeEntry()
	entry.Latency = 123 * time.Millisecond
	logger.Log(entry)

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	latency, ok := result["latency_ms"].(float64)
	if !ok {
		t.Fatalf("latency_ms is not a number, got %T: %v", result["latency_ms"], result["latency_ms"])
	}
	if int(latency) != 123 {
		t.Errorf("expected latency_ms=123, got %v", latency)
	}
}
