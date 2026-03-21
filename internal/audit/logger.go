package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
)

// defaultFileSizeLimit is used when no rotation limit is configured.
const noLimit int64 = 0

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

	// Rotation fields — zero values mean rotation is disabled.
	filePath      string    // path used to reopen/rename on rotation
	maxSizeBytes  int64     // 0 = disabled
	maxAgeHours   int       // 0 = disabled; negative = always trigger
	fileCreatedAt time.Time // when the current file was opened
	bytesWritten  int64     // tracks bytes written to current file
}

// NewLogger creates a Logger from an AuditConfig.
// If Output = "file" or "both" and FilePath is set, opens the file in append mode.
// Rotation is configured from cfg.Rotation (MaxSizeMB and MaxAgeHours).
func NewLogger(cfg *config.AuditConfig) (*Logger, error) {
	l, err := newLogger(cfg, os.Stdout, cfg.FilePath)
	if err != nil {
		return nil, err
	}
	// Wire rotation config from AuditConfig.
	if l.fileHandle != nil {
		l.filePath = cfg.FilePath
		if cfg.Rotation.MaxSizeMB > 0 {
			l.maxSizeBytes = int64(cfg.Rotation.MaxSizeMB) * 1024 * 1024
		}
		l.maxAgeHours = cfg.Rotation.MaxAgeHours
	}
	return l, nil
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

// NewLoggerWithRotation creates a Logger with explicit rotation parameters for testing.
// maxSizeBytes: rotate when file exceeds this many bytes (0 = disabled).
// maxAgeHours: rotate when file is older than this many hours (0 = disabled, negative = already expired).
func NewLoggerWithRotation(cfg *config.AuditConfig, filePath string, maxSizeBytes int64, maxAgeHours int) (*Logger, error) {
	l, err := newLogger(cfg, os.Stdout, filePath)
	if err != nil {
		return nil, err
	}
	l.filePath = filePath
	l.maxSizeBytes = maxSizeBytes
	// For age rotation: store the effective age limit as absolute hours.
	// Negative means "already expired" — set fileCreatedAt far in the past.
	if maxAgeHours < 0 {
		l.maxAgeHours = 1 // 1-hour limit
		l.fileCreatedAt = time.Now().Add(-24 * time.Hour) // file is 24h old — already past limit
	} else {
		l.maxAgeHours = maxAgeHours
	}
	return l, nil
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
		l.fileCreatedAt = time.Now()
		l.writers = []io.Writer{f}

	case "both":
		l.writers = []io.Writer{stdoutWriter}
		if filePath != "" {
			f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("audit: open log file %q: %w", filePath, err)
			}
			l.fileHandle = f
			l.fileCreatedAt = time.Now()
			l.writers = append(l.writers, f)
		}

	default:
		return nil, fmt.Errorf("audit: invalid output mode %q (must be stdout, file, or both)", output)
	}

	return l, nil
}

// shouldRotate returns true if the file should be rotated.
// Must be called with l.mu held.
func (l *Logger) shouldRotate() bool {
	if l.fileHandle == nil {
		return false
	}
	// Check size limit.
	if l.maxSizeBytes > 0 && l.bytesWritten >= l.maxSizeBytes {
		return true
	}
	// Check age limit.
	if l.maxAgeHours != 0 {
		threshold := l.fileCreatedAt.Add(time.Duration(l.maxAgeHours) * time.Hour)
		if time.Now().After(threshold) {
			return true
		}
	}
	return false
}

// rotate closes the current file, renames it to filePath+".1", and opens a new file.
// Must be called with l.mu held.
func (l *Logger) rotate() error {
	if l.fileHandle == nil || l.filePath == "" {
		return nil
	}

	// Close current file.
	if err := l.fileHandle.Close(); err != nil {
		// Continue — best effort on close error.
		_ = err
	}

	// Rename to .1 (simple single-rotation backup).
	rotatedPath := l.filePath + ".1"
	// If .1 already exists, remove it first.
	_ = os.Remove(rotatedPath)
	if err := os.Rename(l.filePath, rotatedPath); err != nil {
		// If rename fails, try to reopen the original file.
		f, openErr := os.OpenFile(l.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if openErr != nil {
			l.fileHandle = nil
			return fmt.Errorf("audit: rotate rename failed: %w", err)
		}
		l.fileHandle = f
		l.fileCreatedAt = time.Now()
		l.bytesWritten = 0
		l.updateFileWriter(f)
		return nil
	}

	// Open a new file at the original path.
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		l.fileHandle = nil
		return fmt.Errorf("audit: open new log file after rotation: %w", err)
	}

	l.fileHandle = f
	l.fileCreatedAt = time.Now()
	l.bytesWritten = 0
	l.updateFileWriter(f)
	return nil
}

// updateFileWriter replaces the file writer in l.writers with the new file handle.
// Must be called with l.mu held.
func (l *Logger) updateFileWriter(newFile *os.File) {
	for i, w := range l.writers {
		if _, isFile := w.(*os.File); isFile {
			l.writers[i] = newFile
			return
		}
	}
}

// Log records an audit entry. If the logger is disabled, it is a no-op.
// Log is safe for concurrent use — the mutex prevents interleaved lines.
func (l *Logger) Log(entry proxy.AuditEntry) {
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
		n, _ := w.Write(line)
		// Track bytes written to file handle for rotation purposes.
		if l.fileHandle != nil {
			if _, isFile := w.(*os.File); isFile {
				l.bytesWritten += int64(n)
			}
		}
	}
	// Check rotation after writing.
	if l.shouldRotate() {
		_ = l.rotate() // Rotation failure is non-fatal — logging continues.
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
