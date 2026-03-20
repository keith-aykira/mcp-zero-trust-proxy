package proxy

import (
	"encoding/json"
	"net/http"
)

// MCPRequest represents a parsed JSON-RPC 2.0 request from an MCP client.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response sent back to an MCP client.
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ClientIdentity holds the authenticated identity of a connected MCP client.
type ClientIdentity struct {
	ClientID  string `json:"client_id"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	Email     string `json:"email"`
}

// ProxyHandler is the core interface implemented by the reverse proxy.
// It handles both regular HTTP requests and SSE (Server-Sent Events) streams.
type ProxyHandler interface {
	// ServeHTTP handles standard JSON-RPC HTTP requests.
	ServeHTTP(w http.ResponseWriter, r *http.Request)

	// HandleSSE handles Server-Sent Events stream connections for streaming MCP transport.
	HandleSSE(w http.ResponseWriter, r *http.Request)
}
