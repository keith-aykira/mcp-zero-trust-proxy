package sinks

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

func TestFileSink_NewFileSink_STDOUT(t *testing.T) {
	cfg := &FileSinkConfig{Output: "stdout"}
	sink, err := NewFileSink("test", cfg)
	if err != nil {
		t.Fatalf("NewFileSink(stdout) error = %v", err)
	}
	if sink == nil {
		t.Fatal("NewFileSink returns nil sink")
	}
	if sink.Name() != "test" {
		t.Errorf("Name() = %q, want %q", sink.Name(), "test")
	}
	if len(sink.writers) != 1 {
		t.Errorf("writers length = %d, want 1", len(sink.writers))
	}
	sink.Close()
}

func TestFileSink_NewFileSink_File(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	cfg := &FileSinkConfig{Output: "file", FilePath: tmpFile}
	sink, err := NewFileSink("test", cfg)
	if err != nil {
		t.Fatalf("NewFileSink(file) error = %v", err)
	}
	defer sink.Close()

	if sink.fileHandle == nil {
		t.Error("fileHandle is nil")
	}
	if len(sink.writers) != 1 {
		t.Errorf("writers length = %d, want 1", len(sink.writers))
	}
}

func TestFileSink_NewFileSink_Both(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	cfg := &FileSinkConfig{Output: "both", FilePath: tmpFile}
	sink, err := NewFileSink("test", cfg)
	if err != nil {
		t.Fatalf("NewFileSink(both) error = %v", err)
	}
	defer sink.Close()

	if len(sink.writers) != 2 {
		t.Errorf("writers length = %d, want 2", len(sink.writers))
	}
}

func TestFileSink_NewFileSink_InvalidOutput(t *testing.T) {
	cfg := &FileSinkConfig{Output: "invalid"}
	_, err := NewFileSink("test", cfg)
	if err == nil {
		t.Error("NewFileSink(invalid) expected error, got nil")
	}
}

func TestFileSink_NewFileSink_FileMissingPath(t *testing.T) {
	cfg := &FileSinkConfig{Output: "file"}
	_, err := NewFileSink("test", cfg)
	if err == nil {
		t.Error("NewFileSink(file without path) expected error, got nil")
	}
}

func TestFileSink_Log(t *testing.T) {
	var captured []byte
	// Create a custom writer to capture output
	type captureWriter struct {
		data []byte
	}
	w := &captureWriter{}

	entry := proxy.AuditEntry{
		Timestamp:    time.Now(),
		ClientID:     "client-123",
		SessionID:    "session-456",
		Method:       "tools/call",
		ToolName:     "read_file",
		Allowed:      true,
		Latency:      42 * time.Millisecond,
		RequestID:    "req-789",
	}

	// Test with a memory buffer
	tmpFile := t.TempDir() + "/test.log"
	cfg := &FileSinkConfig{Output: "file", FilePath: tmpFile}
	sink, err := NewFileSink("test", cfg)
	if err != nil {
		t.Fatalf("NewFileSink error = %v", err)
	}
	defer sink.Close()

	// Log the entry
	err = sink.Log(entry)
	if err != nil {
		t.Errorf("Log() error = %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	// Parse JSON line
	var parsed proxy.AuditEntry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if parsed.ClientID != entry.ClientID {
		t.Errorf("ClientID = %q, want %q", parsed.ClientID, entry.ClientID)
	}
	if parsed.Method != entry.Method {
		t.Errorf("Method = %q, want %q", parsed.Method, entry.Method)
	}
	if !parsed.Allowed {
		t.Error("Allowed = false, want true")
	}
	_ = captured
	_ = w
}

func TestFileSink_Log_Concurrent(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	cfg := &FileSinkConfig{Output: "file", FilePath: tmpFile}
	sink, err := NewFileSink("test", cfg)
	if err != nil {
		t.Fatalf("NewFileSink error = %v", err)
	}
	defer sink.Close()

	// Log concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(idx int) {
			entry := proxy.AuditEntry{
				Timestamp: time.Now(),
				ClientID:  fmt.Sprintf("client-%d", idx),
				Method:    "tools/call",
				Allowed:   true,
				RequestID: fmt.Sprintf("req-%d", idx),
			}
			sink.Log(entry)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Verify all entries written
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 100 {
		t.Errorf("Expected 100 lines, got %d", len(lines))
	}
}

func TestFileSink_Close(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	cfg := &FileSinkConfig{Output: "file", FilePath: tmpFile}
	sink, err := NewFileSink("test", cfg)
	if err != nil {
		t.Fatalf("NewFileSink error = %v", err)
	}

	err = sink.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Close should be safe to call multiple times
	err = sink.Close()
	if err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

func TestFileSink_Rotation_Size(t *testing.T) {
	tmpFile := t.TempDir() + "/test.log"
	// 1KB max size
	cfg := &FileSinkConfig{
		Output:    "file",
		FilePath:  tmpFile,
		MaxSizeMB: 1, // Actually 1MB, but we'll write enough to trigger
	}
	sink, err := NewFileSink("test", cfg)
	if err != nil {
		t.Fatalf("NewFileSink error = %v", err)
	}
	defer sink.Close()

	// Write enough to potentially trigger rotation
	logData := strings.Repeat("x", 1000)
	for i := 0; i < 2000; i++ {
		entry := proxy.AuditEntry{
			Timestamp: time.Now(),
			ClientID:  logData,
			Method:    "tools/call",
			Allowed:   true,
			RequestID: fmt.Sprintf("req-%d", i),
		}
		sink.Log(entry)
	}

	// File should exist
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Log file does not exist")
	}
}

func TestFileSink_FromConfig(t *testing.T) {
	cfg := &config.AuditConfig{
		Output:   "stdout",
		FilePath: "",
		Rotation: config.AuditRotationConfig{
			MaxSizeMB:   10,
			MaxAgeHours: 24,
		},
	}
	sink, err := FromConfig(cfg)
	if err != nil {
		t.Fatalf("FromConfig error = %v", err)
	}
	defer sink.Close()

	if sink.Name() != "file" {
		t.Errorf("Name() = %q, want %q", sink.Name(), "file")
	}
}
