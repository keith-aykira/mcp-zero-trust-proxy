package sinks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
)

// JSONHTTPSink sends raw JSON events over HTTPS to a custom endpoint.
type JSONHTTPSink struct {
	name           string
	enabled        bool
	endpoint       string
	headers        map[string]string
	batchSize      int
	flushInterval  int
	timeout        time.Duration
	bufferSize     int
	basicAuthUser  string
	basicAuthPass  string
	bearerToken    string

	events   chan proxy.AuditEntry
	wg       sync.WaitGroup
	close    chan struct{}
	closed   atomic.Bool
	filter   *Filter
}

// NewJSONHTTPSink creates a new JSON-HTTP sink.
func NewJSONHTTPSink(cfg *config.JSONHTTPSinkConfig) (*JSONHTTPSink, error) {
	if cfg == nil {
		return nil, fmt.Errorf("json_http: config is nil")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("json_http: endpoint is required")
	}

	batchSize := cfg.BatchSize
	if batchSize < 1 {
		batchSize = 50
	}

	flushInterval := cfg.FlushInterval
	if flushInterval < 1 {
		flushInterval = 10
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	bufferSize := cfg.BufferSize
	if bufferSize < 1 {
		bufferSize = 1000
	}

	s := &JSONHTTPSink{
		name:          "json_http",
		enabled:       true,
		endpoint:      cfg.Endpoint,
		headers:       make(map[string]string),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		timeout:       timeout,
		bufferSize:    bufferSize,
	}

	// Copy custom headers
	for k, v := range cfg.Headers {
		s.headers[k] = v
	}

	// Basic auth
	if cfg.BasicAuth != nil {
		s.basicAuthUser = cfg.BasicAuth.Username
		s.basicAuthPass = cfg.BasicAuth.Password
	}

	// Bearer token
	s.bearerToken = cfg.BearerToken

	// Create buffered channel
	s.events = make(chan proxy.AuditEntry, s.bufferSize)
	s.close = make(chan struct{})

	// Start background flusher
	s.wg.Add(1)
	go s.run()

	return s, nil
}

// Name returns the sink name.
func (s *JSONHTTPSink) Name() string {
	return s.name
}

// Log adds an audit entry to the buffer.
func (s *JSONHTTPSink) Log(entry proxy.AuditEntry) error {
	if !s.enabled || s.closed.Load() {
		return nil
	}

	// Apply filter if configured
	if s.filter != nil && !s.filter.Matches(entry) {
		return nil
	}

	// Non-blocking send
	select {
	case s.events <- entry:
		return nil
	default:
		return nil
	}
}

// run is the background goroutine that batches and sends events.
func (s *JSONHTTPSink) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.flushInterval) * time.Second)
	defer ticker.Stop()

	var batch []proxy.AuditEntry

	for {
		select {
		case <-s.close:
			if len(batch) > 0 {
				s.sendBatch(batch)
			}
			return

		case <-ticker.C:
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

// sendBatch sends a batch of events to the configured endpoint.
func (s *JSONHTTPSink) sendBatch(batch []proxy.AuditEntry) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	err := s.sendToEndpoint(ctx, batch)
	if err != nil {
		_ = err
	}
}

// sendToEndpoint sends events to the configured HTTP endpoint.
func (s *JSONHTTPSink) sendToEndpoint(ctx context.Context, entries []proxy.AuditEntry) error {
	// Marshal to JSON
	body, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("json_http: marshal: %w", err)
	}

	client := &http.Client{
		Timeout: s.timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("json_http: create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	// Set authentication
	if s.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.bearerToken)
	} else if s.basicAuthUser != "" {
		req.SetBasicAuth(s.basicAuthUser, s.basicAuthPass)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("json_http: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("json_http: status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SetFilter sets the event filter for this sink.
func (s *JSONHTTPSink) SetFilter(cfg *config.SinkFilterConfig) {
	s.filter = NewFilter(cfg)
}

// Close stops the sink and flushes pending events.
func (s *JSONHTTPSink) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(s.close)
	s.wg.Wait()

	for {
		select {
		case <-s.events:
		default:
			return nil
		}
	}
}
