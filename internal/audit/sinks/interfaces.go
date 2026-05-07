package sinks

import (
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
)

// AuditSink is the interface implemented by all audit sink types.
// Each sink is responsible for its own buffering, batching, and delivery.
type AuditSink interface {
	// Name returns the sink identifier (e.g., "ocsf", "cef", "file").
	Name() string

	// Log writes an audit entry to the sink's destination.
	// Implementations must be non-blocking (use background goroutines/buffers).
	// Returns error only on fatal conditions (not transient network failures).
	Log(entry proxy.AuditEntry) error

	// Close cleanly shuts down the sink (flush buffers, close connections).
	// May block briefly to flush pending events.
	Close() error
}

// SinkFactory creates a new sink instance from configuration.
type SinkFactory func(name string, cfg interface{}) (AuditSink, error)
