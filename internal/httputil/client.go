// Package httputil provides utility functions for HTTP clients and servers with security hardening.
package httputil

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
)

// ClientConfig holds configuration for building outbound HTTP clients.
type ClientConfig struct {
	Timeout       time.Duration
	MinTLSVersion string
}

// NewClient creates an outbound HTTP client with security hardening:
// - Configurable timeout
// - Minimum TLS version enforcement (default: 1.2)
// - Secure default transport configuration
//
// Used for OAuth token exchanges, upstream MCP server connections, and audit sink deliveries.
func NewClient(cfg *config.OutboundConfig, timeout time.Duration) *http.Client {
	if cfg == nil {
		cfg = &config.OutboundConfig{}
	}
	
	// Set default timeout if not provided
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	
	// Build TLS config with minimum version
	tlsConfig := buildOutboundTLSConfig(cfg)
	
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			// Connection pooling settings for efficiency
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// buildOutboundTLSConfig creates a *tls.Config for outbound connections.
// Enforces minimum TLS 1.2 by default to protect against:
// - BEAST attack (TLS 1.0)
// - POODLE attack (SSL/TLS 3.0)
// - Weak cipher suite vulnerabilities
// - Protocol downgrade attacks
func buildOutboundTLSConfig(cfg *config.OutboundConfig) *tls.Config {
	result := &tls.Config{
		MinVersion: tls.VersionTLS12, // Default to TLS 1.2
	}
	
	// Apply configured minimum version
	if cfg != nil && cfg.MinTLSVersion != "" {
		if version, ok := parseTLSVersion(cfg.MinTLSVersion); ok {
			result.MinVersion = version
		}
	}
	
	return result
}

// parseTLSVersion converts a version string to the corresponding TLS constant.
// Returns false for invalid version strings.
func parseTLSVersion(v string) (uint16, bool) {
	switch v {
	case "1.0":
		return tls.VersionTLS10, true
	case "1.1":
		return tls.VersionTLS11, true
	case "1.2":
		return tls.VersionTLS12, true
	case "1.3":
		return tls.VersionTLS13, true
	default:
		return 0, false
	}
}