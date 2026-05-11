package sinks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

// OCFSSink transforms audit entries to OCSF format and sends them to Azure Log Analytics.
type OCFSSink struct {
	name          string
	enabled       bool
	workspaceID   string
	apiKey        string
	batchSize     int
	flushInterval int
	timeout       time.Duration
	bufferSize    int

	events chan proxy.AuditEntry
	wg     sync.WaitGroup
	close  chan struct{}
	closed atomic.Bool
}

// OCSF event fields per specification
const (
	ocsfSchemaVersion = "12.0.0"
)

// NewOCFSSink creates a new OCSF sink configured for Azure Log Analytics.
func NewOCFSSink(cfg *config.OCSFSinkConfig) (*OCFSSink, error) {
	if cfg == nil {
		return nil, fmt.Errorf("ocsf: config is nil")
	}
	if cfg.WorkspaceID == "" {
		return nil, fmt.Errorf("ocsf: workspace_id is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ocsf: api_key is required")
	}

	batchSize := cfg.BatchSize
	if batchSize < 1 {
		batchSize = 100
	}

	flushInterval := cfg.FlushInterval
	if flushInterval < 1 {
		flushInterval = 5
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	bufferSize := cfg.BufferSize
	if bufferSize < 1 {
		bufferSize = 1000
	}

	s := &OCFSSink{
		name:          "ocsf",
		enabled:       true,
		workspaceID:   cfg.WorkspaceID,
		apiKey:        cfg.APIKey,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		timeout:       timeout,
		bufferSize:    bufferSize,
	}

	// Create buffered channel for events
	s.events = make(chan proxy.AuditEntry, s.bufferSize)
	s.close = make(chan struct{})

	// Start background flusher
	s.wg.Add(1)
	go s.run()

	return s, nil
}

// Name returns the sink name.
func (s *OCFSSink) Name() string {
	return s.name
}

// Log adds an audit entry to the buffer for batched delivery.
// Returns nil immediately (non-blocking); events may be dropped if buffer is full.
func (s *OCFSSink) Log(entry proxy.AuditEntry) error {
	if !s.enabled || s.closed.Load() {
		return nil
	}

	// Non-blocking send - drop if buffer full
	select {
	case s.events <- entry:
		return nil
	default:
		// Buffer full - drop event silently to avoid blocking
		return nil
	}
}

// run is the background goroutine that batches and sends events.
func (s *OCFSSink) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.flushInterval) * time.Second)
	defer ticker.Stop()

	var batch []proxy.AuditEntry

	for {
		select {
		case <-s.close:
			// Flush remaining events on close
			if len(batch) > 0 {
				s.sendBatch(batch)
			}
			return

		case <-ticker.C:
			// Flush interval reached
			if len(batch) > 0 {
				s.sendBatch(batch)
				batch = nil
			}

		case entry := <-s.events:
			batch = append(batch, entry)
			if len(batch) >= s.batchSize {
				s.sendBatch(batch)
				batch = nil
			}
		}
	}
}

// sendBatch sends a batch of events to Azure Log Analytics.
func (s *OCFSSink) sendBatch(batch []proxy.AuditEntry) {
	if len(batch) == 0 {
		return
	}

	// Transform events to OCSF format
	ocsfEvents := make([]map[string]interface{}, 0, len(batch))
	for _, entry := range batch {
		ocsfEvent := s.toOCSF(entry)
		ocsfEvents = append(ocsfEvents, ocsfEvent)
	}

	// Send to Azure
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.sendToAzure(ctx, ocsfEvents)
	if err != nil {
		// Log error but don't propagate - don't want to block the proxy
		_ = err
	}
}

// toOCSF transforms an AuditEntry to OCSF format.
func (s *OCFSSink) toOCSF(entry proxy.AuditEntry) map[string]interface{} {
	event := make(map[string]interface{})

	// Set OCSF schema fields
	event["schema_version"] = ocsfSchemaVersion
	event["timestamp"] = entry.Timestamp.UTC().Format(time.RFC3339Nano)

	// Determine event type based on outcome
	if entry.Allowed {
		event["event_category"] = "access"
		event["event_subtype"] = "access_success"
		event["result"] = "success"
	} else {
		event["event_category"] = "access"
		event["event_subtype"] = "access_denied"
		event["result"] = "denied"
	}

	// Device information
	device := make(map[string]interface{})
	device["id"] = fmt.Sprintf("mcp-proxy-%s", s.workspaceID[:8])
	if hostname, err := os.Hostname(); err == nil {
		device["hostname"] = hostname
	}
	event["device"] = device

	// User information
	user := make(map[string]interface{})
	if entry.ClientID != "" {
		user["id"] = entry.ClientID
	}
	if entry.SessionID != "" {
		user["session_id"] = entry.SessionID
	}
	event["user"] = user

	// Target information (the MCP tool/resource being accessed)
	target := make(map[string]interface{})
	if entry.Method != "" {
		target["name"] = entry.Method
	}
	if entry.ToolName != "" {
		target["application"] = make(map[string]interface{})
		target["application"].(map[string]interface{})["name"] = entry.ToolName
	}
	target["category"] = "application"
	event["target"] = target

	// Request details
	detail := make(map[string]interface{})
	if entry.RequestID != "" {
		detail["request_id"] = entry.RequestID
	}
	if entry.LatencyMs > 0 {
		detail["latency_ms"] = entry.LatencyMs
	}
	if !entry.Allowed && entry.DeniedReason != "" {
		detail["denial_reason"] = entry.DeniedReason
	}
	event["detail"] = detail

	return event
}

// sendToAzure sends OCSF events to Azure Log Analytics via the Logs Ingestion API.
func (s *OCFSSink) sendToAzure(ctx context.Context, events []map[string]interface{}) error {
	// Azure Log Analytics endpoint
	url := fmt.Sprintf("https://%s.ods.opinsights.azure.com/api/logs", s.workspaceID)

	// Marshal events to JSON
	body, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("ocsf: marshal events: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: s.timeout,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ocsf: create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ocsf: send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ocsf: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close stops the sink and flushes any pending events.
func (s *OCFSSink) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	close(s.close)
	s.wg.Wait()

	// Drain remaining events from channel
	for {
		select {
		case <-s.events:
			// Discard remaining events
		default:
			return nil
		}
	}
}

// Filter determines if an event should be sent to this sink.
type Filter struct {
	methods   []string
	tools     []string
	results   []string
	clientIDs []string
}

// NewFilter creates a filter from SinkFilterConfig.
func NewFilter(cfg *config.SinkFilterConfig) *Filter {
	if cfg == nil {
		return &Filter{}
	}

	f := &Filter{
		methods:   cfg.Methods,
		tools:     cfg.Tools,
		results:   cfg.Results,
		clientIDs: cfg.ClientIDs,
	}

	return f
}

// Matches returns true if the event passes all filter criteria.
func (f *Filter) Matches(entry proxy.AuditEntry) bool {
	// Filter by method
	if len(f.methods) > 0 {
		found := false
		for _, method := range f.methods {
			if method == entry.Method {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by tool name
	if len(f.tools) > 0 {
		found := false
		for _, pattern := range f.tools {
			if matchString(pattern, entry.ToolName) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by result
	if len(f.results) > 0 {
		found := false
		result := "allowed"
		if !entry.Allowed {
			result = "denied"
		}
		for _, r := range f.results {
			if r == result {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by client ID
	if len(f.clientIDs) > 0 {
		found := false
		for _, clientID := range f.clientIDs {
			if clientID == entry.ClientID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// matchString checks if a string matches a pattern (supports * wildcard).
func matchString(pattern, s string) bool {
	// Convert glob pattern to regex
	pattern = strings.ReplaceAll(pattern, ".", "\\.")
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = "^" + pattern + "$"

	matched, _ := regexp.MatchString(pattern, s)
	return matched
}
