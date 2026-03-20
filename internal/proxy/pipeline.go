package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// Authenticator validates an incoming HTTP request and returns the client identity.
// This interface is defined here (not in the middleware package) to avoid an import
// cycle — middleware imports proxy for MCPRequest/ClientIdentity types.
type Authenticator interface {
	Authenticate(r *http.Request) (*ClientIdentity, error)
}

// RateLimiter enforces per-client request rate limits.
type RateLimiter interface {
	Allow(clientID string) bool
}

// RBACEngine evaluates requests against role-based access control policy and
// filters tools/list responses to only include tools the identity can access.
type RBACEngine interface {
	Process(ctx context.Context, req *MCPRequest, identity *ClientIdentity) (*MCPRequest, error)
	FilterToolsList(identity *ClientIdentity, toolsList json.RawMessage) (json.RawMessage, error)
}

// AuthHandler handles OAuth flow routes (/auth/start, /auth/callback).
type AuthHandler interface {
	HandleAuthStart(w http.ResponseWriter, r *http.Request)
	HandleCallback(w http.ResponseWriter, r *http.Request)
}

// Pipeline orchestrates the middleware chain for incoming MCP requests.
//
// Request flow for MCP traffic:
//  1. Authenticate — validate Bearer token, extract ClientIdentity
//  2. Rate limit — check per-client token bucket
//  3. Parse request — decode JSON-RPC body
//  4. RBAC — enforce method/tool-level policy
//  5. Proxy — forward to upstream MCP server
//  6. Post-proxy — filter tools/list responses through RBAC
//  7. Audit — log outcome (allowed or denied) with latency
//
// Special routes that bypass the pipeline:
//   - GET /health → returns {"status":"ok"}
//   - GET /auth/start → delegated to auth.HandleAuthStart (if authHandler set)
//   - GET /auth/callback → delegated to auth.HandleCallback (if authHandler set)
type Pipeline struct {
	upstream    http.Handler
	auth        Authenticator
	rateLimiter RateLimiter
	rbac        RBACEngine
	auditLogger AuditLogger
	authHandler AuthHandler
}

// NewPipeline constructs a production Pipeline.
// All middleware parameters are optional — pass nil to skip that step.
func NewPipeline(
	upstream http.Handler,
	auth Authenticator,
	rl RateLimiter,
	rbac RBACEngine,
	auditLogger AuditLogger,
	authHandler AuthHandler,
) *Pipeline {
	return &Pipeline{
		upstream:    upstream,
		auth:        auth,
		rateLimiter: rl,
		rbac:        rbac,
		auditLogger: auditLogger,
		authHandler: authHandler,
	}
}

// NewPipelineForTest constructs a Pipeline without an auth handler (for unit tests).
// In tests, the OAuth routes are tested separately; the pipeline behavior under mock
// conditions is the focus.
func NewPipelineForTest(
	upstream http.Handler,
	auth Authenticator,
	rl RateLimiter,
	rbac RBACEngine,
	auditLogger AuditLogger,
) *Pipeline {
	return &Pipeline{
		upstream:    upstream,
		auth:        auth,
		rateLimiter: rl,
		rbac:        rbac,
		auditLogger: auditLogger,
	}
}

// ServeHTTP is the main entry point for all incoming HTTP requests.
// It routes special paths (/health, /auth/*) and runs the security pipeline
// for all other requests.
func (p *Pipeline) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Health check: bypass all middleware.
	if path == "/health" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
		return
	}

	// OAuth flow routes: delegated to auth handler if present.
	if strings.HasPrefix(path, "/auth/") {
		p.handleAuthRoute(w, r)
		return
	}

	// Run the security pipeline for all MCP traffic.
	p.runPipeline(w, r)
}

// handleAuthRoute dispatches /auth/* requests to the OAuth handler.
// If no auth handler is configured, returns 404.
func (p *Pipeline) handleAuthRoute(w http.ResponseWriter, r *http.Request) {
	if p.authHandler == nil {
		http.NotFound(w, r)
		return
	}
	path := r.URL.Path
	switch path {
	case "/auth/start":
		p.authHandler.HandleAuthStart(w, r)
	case "/auth/callback":
		p.authHandler.HandleCallback(w, r)
	default:
		http.NotFound(w, r)
	}
}

// runPipeline executes the full security middleware chain for an MCP request.
func (p *Pipeline) runPipeline(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := generateRequestID()

	// Build an initial audit entry — will be completed after pipeline runs.
	auditEntry := AuditEntry{
		Timestamp: start.UTC(),
		RequestID: requestID,
	}

	// Step 1: Authenticate.
	var identity *ClientIdentity
	if p.auth != nil {
		id, err := p.auth.Authenticate(r)
		if err != nil {
			auditEntry.Allowed = false
			auditEntry.DeniedReason = "auth: " + err.Error()
			auditEntry.Latency = time.Since(start)
			p.logAudit(auditEntry)
			writeJSONRPCError(w, nil, ErrCodeUnauthorized, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
		identity = id
		auditEntry.ClientID = identity.ClientID
		auditEntry.SessionID = identity.SessionID
	}

	// Step 2: Rate limit.
	if p.rateLimiter != nil && identity != nil {
		if !p.rateLimiter.Allow(identity.ClientID) {
			auditEntry.Allowed = false
			auditEntry.DeniedReason = "rate limit exceeded"
			auditEntry.Latency = time.Since(start)
			p.logAudit(auditEntry)
			writeJSONRPCError(w, nil, ErrCodeRateLimited, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	} else if p.rateLimiter != nil && identity == nil {
		// No identity (auth was nil), use a default key
		if !p.rateLimiter.Allow("anonymous") {
			auditEntry.Allowed = false
			auditEntry.DeniedReason = "rate limit exceeded"
			auditEntry.Latency = time.Since(start)
			p.logAudit(auditEntry)
			writeJSONRPCError(w, nil, ErrCodeRateLimited, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
	}

	// Step 3: Parse request body (needed for RBAC and tools/list filtering).
	// Buffer the body so we can both parse it and forward it to upstream.
	var bodyBytes []byte
	var mcpReq *MCPRequest
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()
	}

	if len(bodyBytes) > 0 {
		reqs, err := ParseRequest(bodyBytes)
		if err == nil && len(reqs) > 0 {
			mcpReq = reqs[0]
			auditEntry.Method = mcpReq.Method
			if mcpReq.Method == MethodToolsCall {
				auditEntry.ToolName = ExtractToolName(mcpReq)
			}
		}
	}

	// Restore body for forwarding.
	r = r.WithContext(context.WithValue(r.Context(), MCPRequestKey, mcpReq))
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	r.ContentLength = int64(len(bodyBytes))

	// Step 4: RBAC enforcement.
	if p.rbac != nil && mcpReq != nil {
		if identity == nil {
			identity = &ClientIdentity{ClientID: "anonymous", Role: "readonly"}
		}
		_, err := p.rbac.Process(r.Context(), mcpReq, identity)
		if err != nil {
			auditEntry.Allowed = false
			auditEntry.DeniedReason = "rbac: " + err.Error()
			auditEntry.Latency = time.Since(start)
			p.logAudit(auditEntry)
			writeJSONRPCError(w, mcpReq.ID, ErrCodeForbidden, "Forbidden: "+err.Error(), http.StatusForbidden)
			return
		}
	}

	// Step 5 & 6: Proxy to upstream and post-process tools/list responses.
	// For tools/list, we need to intercept the response to apply RBAC filtering.
	if mcpReq != nil && mcpReq.Method == MethodToolsList && p.rbac != nil && identity != nil {
		// Capture the response for filtering.
		captured := &responseCapture{header: make(http.Header)}
		p.upstream.ServeHTTP(captured, r)

		// Attempt to filter the tools list.
		filtered, err := p.filterToolsListResponse(captured.body.Bytes(), identity)
		if err == nil {
			// Write the filtered response.
			for k, vals := range captured.header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			if captured.statusCode != 0 {
				w.WriteHeader(captured.statusCode)
			}
			w.Write(filtered) //nolint:errcheck
		} else {
			// On filter error, pass through original response.
			for k, vals := range captured.header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			if captured.statusCode != 0 {
				w.WriteHeader(captured.statusCode)
			}
			w.Write(captured.body.Bytes()) //nolint:errcheck
		}
	} else {
		// Standard proxy — forward directly.
		p.upstream.ServeHTTP(w, r)
	}

	// Step 7: Audit log the successful request.
	auditEntry.Allowed = true
	auditEntry.Latency = time.Since(start)
	p.logAudit(auditEntry)
}

// filterToolsListResponse parses an upstream tools/list JSON-RPC response and
// filters its result through the RBAC engine.
func (p *Pipeline) filterToolsListResponse(responseBody []byte, identity *ClientIdentity) ([]byte, error) {
	// Parse the full JSON-RPC response to extract the "result" field.
	var fullResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *RPCError       `json:"error,omitempty"`
	}
	if err := json.Unmarshal(responseBody, &fullResp); err != nil {
		return nil, err
	}

	// If there's no result field (error response), pass through unchanged.
	if fullResp.Result == nil || fullResp.Error != nil {
		return responseBody, nil
	}

	// Filter the tools list in the result.
	filteredResult, err := p.rbac.FilterToolsList(identity, fullResp.Result)
	if err != nil {
		return nil, err
	}

	// Rebuild the full response with the filtered result.
	fullResp.Result = filteredResult
	return json.Marshal(fullResp)
}

// logAudit writes an audit entry if an audit logger is configured.
func (p *Pipeline) logAudit(entry AuditEntry) {
	if p.auditLogger != nil {
		p.auditLogger.Log(entry)
	}
}

// writeJSONRPCError writes a JSON-RPC error response with the given HTTP status code.
func writeJSONRPCError(w http.ResponseWriter, id interface{}, rpcCode int, message string, httpStatus int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    rpcCode,
			Message: message,
		},
	}
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// generateRequestID produces a short unique ID for correlating requests across log entries.
// Uses 8 random bytes encoded as base64url = 11 characters.
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails.
		return "req-" + time.Now().Format("20060102150405")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// responseCapture is a ResponseWriter that buffers the response body and headers
// so the pipeline can inspect and modify the upstream response before forwarding.
type responseCapture struct {
	statusCode int
	header     http.Header
	body       bytes.Buffer
}

func (r *responseCapture) Header() http.Header {
	return r.header
}

func (r *responseCapture) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseCapture) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}
