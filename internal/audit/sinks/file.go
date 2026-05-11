package sinks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

// FileSink writes audit entries to stdout, a file, or both.
// It supports rotation based on file size and age.
type FileSink struct {
	name       string
	enabled    bool
	mu         sync.Mutex
	writers    []io.Writer
	fileHandle *os.File
	filePath   string

	// Rotation fields
	maxSizeBytes  int64
	maxAgeHours   int
	fileCreatedAt time.Time
	bytesWritten  int64
}

// FileSinkConfig holds configuration for FileSink.
type FileSinkConfig struct {
	Output      string
	FilePath    string
	MaxSizeMB   int
	MaxAgeHrs   int
	StdoutWriter io.Writer // Custom writer for stdout (for testing)
}

// NewFileSink creates a new FileSink from configuration.
func NewFileSink(name string, cfg *FileSinkConfig) (*FileSink, error) {
	if name == "" {
		name = "file"
	}

	output := cfg.Output
	if output == "" {
		output = "stdout"
	}

	s := &FileSink{
		name:  name,
		enabled: true,
	}

	switch output {
	case "stdout":
		if cfg.StdoutWriter != nil {
			s.writers = []io.Writer{cfg.StdoutWriter}
		} else {
			s.writers = []io.Writer{os.Stdout}
		}

	case "file":
		if cfg.FilePath == "" {
			return nil, fmt.Errorf("file sink: output=file requires file_path to be set")
		}
		f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("file sink: open log file %q: %w", cfg.FilePath, err)
		}
		s.fileHandle = f
		s.filePath = cfg.FilePath
		s.fileCreatedAt = time.Now()
		s.writers = []io.Writer{f}

	case "both":
		if cfg.StdoutWriter != nil {
			s.writers = []io.Writer{cfg.StdoutWriter}
		} else {
			s.writers = []io.Writer{os.Stdout}
		}
		if cfg.FilePath != "" {
			f, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("file sink: open log file %q: %w", cfg.FilePath, err)
			}
			s.fileHandle = f
			s.filePath = cfg.FilePath
			s.fileCreatedAt = time.Now()
			s.writers = append(s.writers, f)
		}

	default:
		return nil, fmt.Errorf("file sink: invalid output mode %q (must be stdout, file, or both)", output)
	}

	// Configure rotation - negative MaxSizeMB means bytes, positive means MB
	if cfg.MaxSizeMB > 0 {
		s.maxSizeBytes = int64(cfg.MaxSizeMB) * 1024 * 1024
	} else if cfg.MaxSizeMB < 0 {
		// Negative value means bytes (for small rotation limits in tests)
		s.maxSizeBytes = int64(-cfg.MaxSizeMB)
	}
	s.maxAgeHours = cfg.MaxAgeHrs

	return s, nil
}

// Name returns the sink name.
func (s *FileSink) Name() string {
	return s.name
}

// Log writes an audit entry to the configured output.
func (s *FileSink) Log(entry proxy.AuditEntry) error {
	if !s.enabled {
		return nil
	}

	// Get latency - prefer LatencyMs, fall back to Latency for backward compatibility
	latencyMs := entry.LatencyMs
	if latencyMs == 0 && entry.Latency > 0 {
		latencyMs = int64(entry.Latency.Milliseconds())
	}

	// Create a custom struct for JSON marshalling to ensure fields are always
	// present (even when empty) for consistent audit trail
	logEntry := map[string]interface{}{
		"timestamp":     entry.Timestamp.Format(time.RFC3339),
		"client_id":     entry.ClientID,
		"session_id":    entry.SessionID,
		"method":        entry.Method,
		"allowed":       entry.Allowed,
		"denied_reason": entry.DeniedReason,
		"latency_ms":    latencyMs,
		"request_id":    entry.RequestID,
	}
	if entry.ToolName != "" {
		logEntry["tool_name"] = entry.ToolName
	}

	// Marshal to JSON
	line, err := json.Marshal(logEntry)
	if err != nil {
		// Fail silently - audit logging should not crash the proxy
		return nil
	}
	line = append(line, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, w := range s.writers {
		n, _ := w.Write(line)
		// Track bytes written for rotation
		if s.fileHandle != nil {
			if _, isFile := w.(*os.File); isFile {
				s.bytesWritten += int64(n)
			}
		}
	}

	// Check rotation after writing
	if s.shouldRotate() {
		_ = s.rotate()
	}

	return nil
}

// shouldRotate returns true if the file should be rotated.
// Must be called with s.mu held.
func (s *FileSink) shouldRotate() bool {
	if s.fileHandle == nil {
		return false
	}
	// Check size limit
	if s.maxSizeBytes > 0 && s.bytesWritten >= s.maxSizeBytes {
		return true
	}
	// Check age limit
	if s.maxAgeHours != 0 {
		threshold := s.fileCreatedAt.Add(time.Duration(s.maxAgeHours) * time.Hour)
		if time.Now().After(threshold) {
			return true
		}
	}
	return false
}

// rotate closes the current file, renames it, and opens a new file.
// Must be called with s.mu held.
func (s *FileSink) rotate() error {
	if s.fileHandle == nil || s.filePath == "" {
		return nil
	}

	// Close current file
	_ = s.fileHandle.Close()

	// Rename to .1
	rotatedPath := s.filePath + ".1"
	_ = os.Remove(rotatedPath)
	if err := os.Rename(s.filePath, rotatedPath); err != nil {
		// Try to reopen original file
		f, openErr := os.OpenFile(s.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if openErr != nil {
			s.fileHandle = nil
			return fmt.Errorf("file sink: rotate rename failed: %w", err)
		}
		s.fileHandle = f
		s.fileCreatedAt = time.Now()
		s.bytesWritten = 0
		s.updateFileWriter(f)
		return nil
	}

	// Open new file
	f, err := os.OpenFile(s.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		s.fileHandle = nil
		return fmt.Errorf("file sink: open new log file after rotation: %w", err)
	}

	s.fileHandle = f
	s.fileCreatedAt = time.Now()
	s.bytesWritten = 0
	s.updateFileWriter(f)
	return nil
}

// updateFileWriter replaces the file writer in s.writers with the new file handle.
// Must be called with s.mu held.
func (s *FileSink) updateFileWriter(newFile *os.File) {
	for i, w := range s.writers {
		if _, isFile := w.(*os.File); isFile {
			s.writers[i] = newFile
			return
		}
	}
}

// Close flushes and closes the file handle.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fileHandle != nil {
		err := s.fileHandle.Close()
		s.fileHandle = nil
		return err
	}
	return nil
}

// FromConfig creates a FileSink from AuditConfig (for backward compatibility).
func FromConfig(cfg *config.AuditConfig) (*FileSink, error) {
	return FromConfigWithWriter(cfg, nil)
}

// FromConfigWithWriter creates a FileSink from AuditConfig with a custom stdout writer.
func FromConfigWithWriter(cfg *config.AuditConfig, stdoutWriter interface{}) (*FileSink, error) {
	fileCfg := &FileSinkConfig{
		Output:    cfg.Output,
		FilePath:  cfg.FilePath,
		MaxSizeMB: cfg.Rotation.MaxSizeMB,
		MaxAgeHrs: cfg.Rotation.MaxAgeHours,
	}
	if stdoutWriter != nil {
		if w, ok := stdoutWriter.(io.Writer); ok {
			fileCfg.StdoutWriter = w
		}
	}
	return NewFileSink("file", fileCfg)
}
