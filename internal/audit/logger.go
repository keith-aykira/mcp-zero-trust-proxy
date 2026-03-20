package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/middleware"
)

// logEntry is the JSON-serializable representation of an audit entry.
// We use a separate struct (rather than marshaling middleware.AuditEntry directly)
// so we can control the exact JSON field names and latency format.
type logEntry struct {
	Timestamp    string `json:"timestamp"`
	ClientID     string `json:"client_id"`
	SessionID    string `json:"session_id"`
	Method       string `json:"method"`
	ToolName     string `json:"tool_name"`
	Allowed      bool   `json:"allowed"`
	DeniedReason string `json:"denied_reason"`
	LatencyMs    int64  `json:"latency_ms"`
	RequestID    string `json:"request_id"`
}

// Logger writes structured JSON audit entries (JSONL format) to stdout, a file, or both.
// It implements the middleware.AuditLogger interface.
// Logger is safe for concurrent use.
type Logger struct {
	enabled bool
	mu      sync.Mutex
	// writers holds all output destinations (stdout writer and/or file).
	writers []io.Writer
	// fileHandle is the open file (if file output is configured). May be nil.
	fileHandle *os.File
}

// NewLogger creates a Logger from an AuditConfig.
// If Output = "file" or "both" and FilePath is set, opens the file in append mode.
func NewLogger(cfg *config.AuditConfig) (*Logger, error) {
	return newLogger(cfg, os.Stdout, cfg.FilePath)
}

// NewLoggerWithWriter creates a Logger that writes to the provided writer instead of os.Stdout.
// Useful for testing — inject a bytes.Buffer to capture output.
func NewLoggerWithWriter(cfg *config.AuditConfig, w io.Writer) (*Logger, error) {
	return newLogger(cfg, w, "")
}

// NewLoggerWithWriterAndFile creates a Logger using a custom stdout writer and an explicit file path.
// Used by tests that need to verify both outputs simultaneously.
func NewLoggerWithWriterAndFile(cfg *config.AuditConfig, w io.Writer, filePath string) (*Logger, error) {
	return newLogger(cfg, w, filePath)
}

// newLogger is the internal constructor shared by all public constructors.
func newLogger(cfg *config.AuditConfig, stdoutWriter io.Writer, filePath string) (*Logger, error) {
	l := &Logger{
		enabled: cfg.Enabled,
	}

	if !cfg.Enabled {
		return l, nil
	}

	output := cfg.Output
	if output == "" {
		output = "stdout"
	}

	switch output {
	case "stdout":
		l.writers = []io.Writer{stdoutWriter}

	case "file":
		if filePath == "" {
			return nil, fmt.Errorf("audit: output=file requires file_path to be set")
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("audit: open log file %q: %w", filePath, err)
		}
		l.fileHandle = f
		l.writers = []io.Writer{f}

	case "both":
		l.writers = []io.Writer{stdoutWriter}
		if filePath != "" {
			f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("audit: open log file %q: %w", filePath, err)
			}
			l.fileHandle = f
			l.writers = append(l.writers, f)
		}

	default:
		return nil, fmt.Errorf("audit: invalid output mode %q (must be stdout, file, or both)", output)
	}

	return l, nil
}

// Log records an audit entry. If the logger is disabled, it is a no-op.
// Log is safe for concurrent use — the mutex prevents interleaved lines.
func (l *Logger) Log(entry middleware.AuditEntry) {
	if !l.enabled {
		return
	}

	le := logEntry{
		Timestamp:    entry.Timestamp.UTC().Format(time.RFC3339),
		ClientID:     entry.ClientID,
		SessionID:    entry.SessionID,
		Method:       entry.Method,
		ToolName:     entry.ToolName,
		Allowed:      entry.Allowed,
		DeniedReason: entry.DeniedReason,
		LatencyMs:    entry.Latency.Milliseconds(),
		RequestID:    entry.RequestID,
	}

	line, err := json.Marshal(le)
	if err != nil {
		// Marshal failure should never happen for this struct, but fail silently
		// rather than panic — audit logging must not crash the proxy.
		return
	}
	line = append(line, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	for _, w := range l.writers {
		_, _ = w.Write(line)
	}
}

// Close flushes and closes the file handle if open.
// It is safe to call Close multiple times.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fileHandle != nil {
		err := l.fileHandle.Close()
		l.fileHandle = nil
		return err
	}
	return nil
}
