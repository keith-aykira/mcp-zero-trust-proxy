package audit

import (
	"fmt"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

// Logger is an alias for Engine for backward compatibility.
// It implements proxy.AuditLogger interface.
type Logger = Engine

// NewLogger creates a new audit logger from configuration.
// This is the main entry point for creating audit loggers.
func NewLogger(cfg *config.AuditConfig) (*Logger, error) {
	return NewEngine(cfg)
}

// NewLoggerWithWriter creates a logger for testing purposes with a custom stdout writer.
func NewLoggerWithWriter(cfg *config.AuditConfig, w interface{}) (*Logger, error) {
	if cfg == nil {
		cfg = &config.AuditConfig{
			Enabled: true,
			Output:  "stdout",
		}
	}
	return NewEngineWithWriter(cfg, w)
}

// NewLoggerWithWriterAndFile creates a logger for testing with a custom stdout writer and file output.
func NewLoggerWithWriterAndFile(cfg *config.AuditConfig, w interface{}, filePath string) (*Logger, error) {
	if cfg == nil {
		cfg = &config.AuditConfig{
			Enabled:  true,
			Output:   "both",
			FilePath: filePath,
		}
	}
	return NewEngineWithWriter(cfg, w)
}

// NewLoggerWithRotation creates a logger with explicit rotation parameters.
// Deprecated: Use NewEngine with AuditConfig.Rotation.
func NewLoggerWithRotation(cfg *config.AuditConfig, filePath string, maxSizeBytes int64, maxAgeHours int) (*Logger, error) {
	if cfg == nil {
		cfg = &config.AuditConfig{
			Enabled:  true,
			Output:   "file",
			FilePath: filePath,
		}
	}
	// Configure rotation - use bytes directly for small sizes, convert to MB for larger ones
	if maxSizeBytes > 0 {
		if maxSizeBytes < 1024*1024 {
			// For sizes less than 1 MB, store as bytes (negative value to indicate bytes)
			cfg.Rotation.MaxSizeMB = -int(maxSizeBytes)
		} else {
			cfg.Rotation.MaxSizeMB = int(maxSizeBytes / 1024 / 1024)
			if cfg.Rotation.MaxSizeMB < 1 {
				cfg.Rotation.MaxSizeMB = 1
			}
		}
	}
	cfg.Rotation.MaxAgeHours = maxAgeHours
	return NewEngine(cfg)
}

// Ensure Logger implements proxy.AuditLogger
var _ proxy.AuditLogger = (*Logger)(nil)

// Helper function for migration path
func MigrateFromLegacyConfig(oldEnabled bool, oldOutput string, oldFilePath string, oldMaxSizeMB int, oldMaxAgeHours int) *config.AuditConfig {
	cfg := &config.AuditConfig{
		Enabled:  oldEnabled,
		Output:   oldOutput,
		FilePath: oldFilePath,
		Rotation: config.AuditRotationConfig{
			MaxSizeMB:   oldMaxSizeMB,
			MaxAgeHours: oldMaxAgeHours,
		},
	}
	return cfg
}

// ValidationError is returned when configuration is invalid
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("audit config %s: %s", e.Field, e.Msg)
}
