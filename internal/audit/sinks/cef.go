package sinks

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

// CEFSink sends audit entries in CEF (Common Event Format) to syslog servers.
// Supports UDP, TCP, TCP-TLS, and HTTPS transports.
type CEFSink struct {
	name          string
	enabled       bool
	transport     string
	host          string
	port          int
	facility      string
	batchSize     int
	flushInterval int
	timeout       time.Duration
	bufferSize    int

	events   chan proxy.AuditEntry
	wg       sync.WaitGroup
	close    chan struct{}
	closed   atomic.Bool
	filter   *Filter

	// UDP connector
	udpConn net.Conn

	// HTTP client for HTTPS transport
	httpClient *http.Client
}

// CEF constants
const (
	cefVersion     = "0"
	cefVendor      = "MCPZeroTrust"
	cefProduct     = "Proxy"
	cefVersionNum  = "1.0"
)

// syslog facility mapping
var facilityNames = map[string]int{
	"kernel":     0,
	"user":       1,
	"mail":       2,
	"daemon":     3,
	"auth":       4,
	"syslog":     5,
	"lpr":        6,
	"news":       7,
	"uucp":       8,
	"cron":       9,
	"authpriv":   10,
	"ftp":        11,
	"local0":     16,
	"local1":     17,
	"local2":     18,
	"local3":     19,
	"local4":     20,
	"local5":     21,
	"local6":     22,
	"local7":     23,
}

// NewCEFSink creates a new CEF sink with the specified transport.
func NewCEFSink(cfg *config.CEFSinkConfig) (*CEFSink, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cef: config is nil")
	}
	if cfg.Host == "" {
		return nil, fmt.Errorf("cef: host is required")
	}

	transport := cfg.Transport
	if transport == "" {
		transport = "udp"
	}

	port := cfg.Port
	if port == 0 {
		switch transport {
		case "tcp_tls":
			port = 6514
		case "udp", "tcp":
			port = 514
		case "https":
			port = 443
		}
	}

	facility := cfg.Facility
	if facility == "" {
		facility = "local0"
	}

	// Validate facility
	if _, ok := facilityNames[facility]; !ok {
		return nil, fmt.Errorf("cef: unknown facility %q", facility)
	}

	batchSize := cfg.BatchSize
	if batchSize < 1 && transport == "https" {
		batchSize = 10
	}

	flushInterval := cfg.FlushInterval
	if flushInterval < 1 && transport == "https" {
		flushInterval = 1
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	bufferSize := cfg.BufferSize
	if bufferSize < 1 {
		bufferSize = 1000
	}

	s := &CEFSink{
		name:          "cef",
		enabled:       true,
		transport:     transport,
		host:          cfg.Host,
		port:          port,
		facility:      facility,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		timeout:       timeout,
		bufferSize:    bufferSize,
	}

	// Setup UDP connection immediately
	if transport == "udp" {
		addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))
		conn, err := net.DialTimeout("udp", addr, s.timeout)
		if err != nil {
			return nil, fmt.Errorf("cef: dial UDP: %w", err)
		}
		s.udpConn = conn
	}

	// Setup HTTP client for HTTPS transport
	if transport == "https" {
		s.httpClient = &http.Client{
			Timeout: s.timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ServerName: s.host,
				},
			},
		}
	}

	// Create buffered channel
	s.events = make(chan proxy.AuditEntry, s.bufferSize)
	s.close = make(chan struct{})

	// Start background processor
	s.wg.Add(1)
	go s.run()

	return s, nil
}

// Name returns the sink name.
func (s *CEFSink) Name() string {
	return s.name
}

// Log adds an audit entry to the buffer.
func (s *CEFSink) Log(entry proxy.AuditEntry) error {
	if !s.enabled || s.closed.Load() {
		return nil
	}

	// Apply filter if configured
	if s.filter != nil && !s.filter.Matches(entry) {
		return nil
	}

	// For UDP/tcp transports (non-HTTPS), send immediately
	if s.transport != "https" {
		return s.sendCEF(s.toCEF(entry))
	}

	// For HTTPS, buffer for batching
	select {
	case s.events <- entry:
		return nil
	default:
		return nil
	}
}

// run processes events for HTTPS transport (batching).
func (s *CEFSink) run() {
	defer s.wg.Done()

	if s.transport != "https" {
		// For non-HTTPS, just wait for close
		<-s.close
		return
	}

	ticker := time.NewTicker(time.Duration(s.flushInterval) * time.Second)
	defer ticker.Stop()

	var batch []proxy.AuditEntry

	for {
		select {
		case <-s.close:
			if len(batch) > 0 {
				s.sendCEFBatch(batch)
			}
			return

		case <-ticker.C:
			if len(batch) > 0 {
				s.sendCEFBatch(batch)
				batch = nil
			}

		case entry := <-s.events:
			batch = append(batch, entry)
			if len(batch) >= s.batchSize {
				s.sendCEFBatch(batch)
				batch = nil
			}
		}
	}
}

// toCEF converts an audit entry to CEF format.
func (s *CEFSink) toCEF(entry proxy.AuditEntry) string {
	// Determine severity and event class
	severity := 1
	deviceEventClass := "OTHER::Activity"
	name := "Activity"

	if entry.Allowed {
		if entry.Method == "tools/call" {
			deviceEventClass = "AUTH::Authentication"
			name = "AUTH:Authentication"
		}
	} else {
		severity = 8
		deviceEventClass = "AUTH::AccessDenied"
		name = "AUTH:Authorization"
	}

	// Build extension key-value pairs
	var extBuilder strings.Builder
	extBuilder.WriteString(fmt.Sprintf("src=%s", s.getClientIP(entry)))
	extBuilder.WriteString(fmt.Sprintf(" dstPort=%d", 8080))
	if entry.ClientID != "" {
		extBuilder.WriteString(fmt.Sprintf(" client_id=%s", entry.ClientID))
	}
	if entry.SessionID != "" {
		extBuilder.WriteString(fmt.Sprintf(" session_id=%s", entry.SessionID))
	}
	extBuilder.WriteString(fmt.Sprintf(" user=%s", sanitizeCEF(entry.ClientID)))
	extBuilder.WriteString(fmt.Sprintf(" action=%s", map[bool]string{true: "allow", false: "deny"}[entry.Allowed]))
	if entry.ToolName != "" {
		extBuilder.WriteString(fmt.Sprintf(" tool=%s", sanitizeCEF(entry.ToolName)))
	}
	extBuilder.WriteString(fmt.Sprintf(" method=%s", entry.Method))
	if entry.LatencyMs > 0 {
		extBuilder.WriteString(fmt.Sprintf(" latency_ms=%d", entry.LatencyMs))
	}
	extBuilder.WriteString(fmt.Sprintf(" request_id=%s", entry.RequestID))
	if !entry.Allowed && entry.DeniedReason != "" {
		extBuilder.WriteString(fmt.Sprintf(" reason=%s", sanitizeCEF(entry.DeniedReason)))
	}

	// Build CEF message
	// CEF:Version|Device Vendor|Device Product|Device Version|Device Event Class ID|Name|Device Event Class|Severity|Extension
	return fmt.Sprintf(
		"CEF:%s|%s|%s|%s|%s|%s|%s|%d|%s",
		cefVersion,
		cefVendor,
		cefProduct,
		cefVersionNum,
		"", // Device Extended ID (empty)
		name,
		deviceEventClass,
		severity,
		extBuilder.String(),
	)
}

// getClientIP extracts client IP from the entry.
func (s *CEFSink) getClientIP(entry proxy.AuditEntry) string {
	if entry.ClientIP != "" {
		return entry.ClientIP
	}
	return "0.0.0.0"
}

// sanitizeCEF escapes special characters in CEF extension values per CEF 0 specification.
// CEF fields delimited by '|' and extension key=value pairs use '=' as delimiter.
// Control characters (CRLF) are removed to prevent log injection. Colons and backslashes
// are escaped to maintain proper CEF parsing. Spaces are replaced for readability.
func sanitizeCEF(s string) string {
	s = strings.ReplaceAll(s, "\\", `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "=", `\=`)
	s = strings.ReplaceAll(s, ":", `\:`)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// sendCEF sends a single CEF message (UDP/TCP).
func (s *CEFSink) sendCEF(message string) error {
	msgBytes := []byte(message + "\n")

	switch s.transport {
	case "udp":
		if s.udpConn != nil {
			_, err := s.udpConn.Write(msgBytes)
			return err
		}

	case "tcp", "tcp_tls":
		conn, err := s.dialTCP()
		if err != nil {
			return err
		}
		defer conn.Close()
		_, err = conn.Write(msgBytes)
		return err

	default:
		return nil
	}

	return nil
}

// dialTCP creates a TCP or TCP-TLS connection.
func (s *CEFSink) dialTCP() (net.Conn, error) {
	addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))

	if s.transport == "tcp_tls" {
		return tls.DialWithDialer(
			&net.Dialer{Timeout: s.timeout},
			"tcp",
			addr,
			&tls.Config{
				ServerName: s.host,
			},
		)
	}

	return net.DialTimeout("tcp", addr, s.timeout)
}

// sendCEFBatch sends a batch of CEF messages via HTTPS.
func (s *CEFSink) sendCEFBatch(batch []proxy.AuditEntry) {
	if len(batch) == 0 {
		return
	}

	// Convert to JSON (CEF over HTTPS typically uses JSON array)
	events := make([]map[string]interface{}, 0, len(batch))
	for _, entry := range batch {
		cefMsg := s.toCEF(entry)
		events = append(events, map[string]interface{}{
			"message": cefMsg,
			"timestamp": entry.Timestamp.UTC().Format(time.RFC3339),
		})
	}

	body, _ := json.Marshal(events)

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://%s:%d/syslog", s.host, s.port), bytes.NewReader(body))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// SetFilter sets the event filter.
func (s *CEFSink) SetFilter(cfg *config.SinkFilterConfig) {
	s.filter = NewFilter(cfg)
}

// Close stops the sink and cleans up resources.
func (s *CEFSink) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(s.close)
	s.wg.Wait()

	// Close UDP connection
	if s.udpConn != nil {
		s.udpConn.Close()
	}

	// Drain channel
	for {
		select {
		case <-s.events:
		default:
			return nil
		}
	}
}
