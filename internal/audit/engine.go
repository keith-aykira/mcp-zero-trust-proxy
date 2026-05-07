package audit

import (
	"fmt"
	"sync"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/audit/sinks"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
)

// Engine orchestrates multiple audit sinks, fan-out ing events to all configured sinks.
// It implements the proxy.AuditLogger interface for seamless integration with the pipeline.
type Engine struct {
	enabled bool
	mu      sync.RWMutex
	sinks   []sinks.AuditSink
}

// NewEngine creates a new audit engine from configuration.
// It creates both the legacy file sink (for backward compatibility) and any new external sinks.
func NewEngine(cfg *config.AuditConfig) (*Engine, error) {
	return NewEngineWithWriter(cfg, nil)
}

// NewEngineWithWriter creates a new audit engine with a custom stdout writer for testing.
func NewEngineWithWriter(cfg *config.AuditConfig, stdoutWriter interface{}) (*Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("audit: config is nil")
	}

	e := &Engine{
		enabled: cfg.Enabled,
		sinks:   make([]sinks.AuditSink, 0, 1+len(cfg.Sinks)),
	}

	// Create legacy file sink for backward compatibility
	if cfg.Enabled && (cfg.Output == "stdout" || cfg.Output == "file" || cfg.Output == "both") {
		var writer interface{} = nil
		if stdoutWriter != nil {
			writer = stdoutWriter
		}
		fileSink, err := sinks.FromConfigWithWriter(cfg, writer)
		if err != nil {
			return nil, fmt.Errorf("audit: create file sink: %w", err)
		}
		e.sinks = append(e.sinks, fileSink)
	}

	// Create external sinks
	for i, sinkCfg := range cfg.Sinks {
		if !sinkCfg.Enabled {
			continue
		}

		sink, err := createExternalSink(&sinkCfg)
		if err != nil {
			return nil, fmt.Errorf("audit: create sink[%d] (type=%q): %w", i, sinkCfg.Type, err)
		}
		e.sinks = append(e.sinks, sink)
	}

	return e, nil
}

// createExternalSink creates an external sink from configuration.
func createExternalSink(cfg *config.AuditSinkConfig) (sinks.AuditSink, error) {
	switch cfg.Type {
	case "ocsf":
		if cfg.OCSF == nil {
			return nil, fmt.Errorf("ocsf config required")
		}
		return sinks.NewOCFSSink(cfg.OCSF)

	case "cef":
		if cfg.CEF == nil {
			return nil, fmt.Errorf("cef config required")
		}
		sink, err := sinks.NewCEFSink(cfg.CEF)
		if err != nil {
			return nil, err
		}
		// Apply filter
		if cfg.Filter != nil {
			sink.SetFilter(cfg.Filter)
		}
		return sink, nil

	case "json_http":
		if cfg.JSONHTTP == nil {
			return nil, fmt.Errorf("json_http config required")
		}
		sink, err := sinks.NewJSONHTTPSink(cfg.JSONHTTP)
		if err != nil {
			return nil, err
		}
		// Apply filter
		if cfg.Filter != nil {
			sink.SetFilter(cfg.Filter)
		}
		return sink, nil

	default:
		return nil, fmt.Errorf("unknown sink type: %q", cfg.Type)
	}
}

// Log implements proxy.AuditLogger. It fans out the entry to all sinks.
// Sink errors are ignored to prevent audit logging from affecting request processing.
func (e *Engine) Log(entry proxy.AuditEntry) {
	if !e.enabled {
		return
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Fan-out to all sinks (non-blocking)
	for _, sink := range e.sinks {
		_ = sink.Log(entry)
	}
}

// AddSink adds an additional sink at runtime.
func (e *Engine) AddSink(sink sinks.AuditSink) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sinks = append(e.sinks, sink)
}

// RemoveSink removes a sink by name.
func (e *Engine) RemoveSink(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, sink := range e.sinks {
		if sink.Name() == name {
			e.sinks = append(e.sinks[:i], e.sinks[i+1:]...)
			return
		}
	}
}

// Close shuts down all sinks gracefully.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var firstErr error
	for _, sink := range e.sinks {
		if err := sink.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Enabled returns whether the engine is enabled.
func (e *Engine) Enabled() bool {
	return e.enabled
}

// SetEnabled enables or disables the engine at runtime.
func (e *Engine) SetEnabled(enabled bool) {
	e.enabled = enabled
}

// SinkCount returns the number of configured sinks.
func (e *Engine) SinkCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.sinks)
}
