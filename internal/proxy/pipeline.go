package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// CORSConfig controls Cross-Origin Resource Sharing headers applied by the pipeline.
// When AllowedOrigins is empty, no CORS headers are added.
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int // seconds
}

// PipelineOption is a functional option for configuring a Pipeline.
type PipelineOption func(*Pipeline)

// WithMaxBodySize sets the maximum request body size in bytes.
// Requests exceeding this limit are rejected with 413 before any parsing.
func WithMaxBodySize(n int64) PipelineOption {
	return func(p *Pipeline) {
		p.maxBodySize = n
	}
}

// WithCORS configures CORS header injection for the pipeline.
func WithCORS(cfg *CORSConfig) PipelineOption {
	return func(p *Pipeline) {
		p.corsConfig = cfg
	}
}

// Pipeline orchestrates the middleware chain for incoming MCP requests.
//
// Request flow for MCP traffic:
//  1. CORS preflight — handle OPTIONS before auth
//  2. Body size enforcement — reject oversized bodies before parsing
//  3. Authenticate — validate Bearer token, extract ClientIdentity
//  4. Rate limit — check per-client token bucket
//  5. Parse request — decode JSON-RPC body (single parse, shared with upstream)
//  6. RBAC — enforce method/tool-level policy
//  7. Proxy — forward to upstream MCP server
//  8. Post-proxy — filter tools/list responses through RBAC
//  9. Audit — log outcome (allowed or denied) with latency
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
	maxBodySize int64
	corsConfig  *CORSConfig
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
	opts ...PipelineOption,
) *Pipeline {
	p := &Pipeline{
		upstream:    upstream,
		auth:        auth,
		rateLimiter: rl,
		rbac:        rbac,
		auditLogger: auditLogger,
		authHandler: authHandler,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
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
	opts ...PipelineOption,
) *Pipeline {
	p := &Pipeline{
		upstream:    upstream,
		auth:        auth,
		rateLimiter: rl,
		rbac:        rbac,
		auditLogger: auditLogger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ServeHTTP is the main entry point for all incoming HTTP requests.
// It routes special paths (/health, /auth/*) and runs the security pipeline
// for all other requests.
func (p *Pipeline) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Apply CORS headers before routing — all responses get them.
	p.applyCORS(w, r)

	// Handle OPTIONS preflight — respond without running the pipeline.
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

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

// applyCORS sets Cross-Origin Resource Sharing headers on the response.
// If corsConfig is nil or AllowedOrigins is empty, no headers are added.
func (p *Pipeline) applyCORS(w http.ResponseWriter, r *http.Request) {
	if p.corsConfig == nil || len(p.corsConfig.AllowedOrigins) == 0 {
		return
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	// Check if the request origin is in the allowed list.
	for _, allowed := range p.corsConfig.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if len(p.corsConfig.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(p.corsConfig.AllowedMethods, ", "))
			}
			if len(p.corsConfig.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(p.corsConfig.AllowedHeaders, ", "))
			}
			if p.corsConfig.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", p.corsConfig.MaxAge))
			}
			break
		}
	}
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

	// Set X-Request-ID header before any writes so both success and error responses include it.
	w.Header().Set("X-Request-ID", requestID)

	// Build an initial audit entry — will be completed after pipeline runs.
	auditEntry := AuditEntry{
		Timestamp: start.UTC(),
		RequestID: requestID,
	}

	// Step 0: Enforce body size limit BEFORE any parsing.
	// Check ContentLength first for known sizes; for chunked transfers (ContentLength=-1),
	// wrap the body with LimitReader and check after reading.
	if p.maxBodySize > 0 && r.ContentLength > p.maxBodySize {
		p.logAudit(AuditEntry{
			Timestamp:    start.UTC(),
			RequestID:    requestID,
			Allowed:      false,
			DeniedReason: "request body too large",
			Latency:      time.Since(start),
		})
		writeJSONRPCError(w, nil, ErrCodeRequestTooLarge, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	// Wrap body with LimitReader for chunked transfers (ContentLength=-1).
	// Read limit is maxBodySize+1 so we can detect overflow.
	if p.maxBodySize > 0 && r.Body != nil {
		r.Body = io.NopCloser(io.LimitReader(r.Body, p.maxBodySize+1))
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

	// Step 2b: SSE role enforcement.
	// SSE streams bypass per-response RBAC inspection — the upstream can emit
	// responses for any method once the connection is open. Until per-event
	// filtering is implemented (Option A), only the admin role may open SSE
	// connections. Restricted/readonly clients must use the standard JSON-RPC
	// request/response path, which is fully protected by RBAC.
	if isSSERequest(r) {
		role := ""
		if identity != nil {
			role = identity.Role
		}
		if role != "admin" {
			auditEntry.Allowed = false
			auditEntry.DeniedReason = "sse: role not permitted"
			auditEntry.Latency = time.Since(start)
			p.logAudit(auditEntry)
			writeJSONRPCError(w, nil, ErrCodeForbidden, "SSE streaming requires admin role", http.StatusForbidden)
			return
		}
	}

	// Step 3: Parse request body (needed for RBAC and tools/list filtering).
	// Step 3: Parse request body — single parse, shared with upstream via context.
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
		// LimitReader overflow check for chunked transfers.
		if p.maxBodySize > 0 && int64(len(bodyBytes)) > p.maxBodySize {
			auditEntry.Allowed = false
			auditEntry.DeniedReason = "request body too large"
			auditEntry.Latency = time.Since(start)
			p.logAudit(auditEntry)
			writeJSONRPCError(w, nil, ErrCodeRequestTooLarge, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
	}

	if len(bodyBytes) > 0 {
		// Check for batch before parsing — an empty batch [] still needs batch handling.
		if isBatchRequest(bodyBytes) {
			reqs, err := ParseRequest(bodyBytes)
			if err != nil {
				writeJSONRPCError(w, nil, ErrCodeParseError, "Parse error: "+err.Error(), http.StatusBadRequest)
				return
			}
			p.processBatch(w, r, reqs, identity, start, requestID)
			return
		}

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
			// Write the filtered response with corrected Content-Length.
			for k, vals := range captured.header {
				if k == "Content-Length" {
					continue // will be set to match the filtered body size
				}
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(filtered)))
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

// isBatchRequest returns true if the raw body is a JSON array (batch JSON-RPC request).
func isBatchRequest(body []byte) bool {
	trimmed := trimLeftSpace(body)
	return len(trimmed) > 0 && trimmed[0] == '['
}

// processBatch handles a batch JSON-RPC request by applying per-item RBAC enforcement.
//
// For each item in the batch:
//   - RBAC is checked independently.
//   - If denied: a JSON-RPC error response is built for that item.
//   - If allowed: the item is forwarded to upstream individually, and the response is collected.
//   - An audit entry is logged for each item.
//
// The final batch response is a JSON array matching the order and IDs of the input items.
func (p *Pipeline) processBatch(w http.ResponseWriter, r *http.Request, reqs []*MCPRequest, identity *ClientIdentity, start time.Time, requestID string) {
	ctx := r.Context()

	// If no identity from auth, use anonymous for RBAC checks.
	if identity == nil && p.rbac != nil {
		identity = &ClientIdentity{ClientID: "anonymous", Role: "readonly"}
	}

	responses := make([]json.RawMessage, len(reqs))

	for i, req := range reqs {
		itemStart := time.Now()
		auditEntry := AuditEntry{
			Timestamp: itemStart.UTC(),
			RequestID: requestID,
			Method:    req.Method,
		}
		if req.Method == MethodToolsCall {
			auditEntry.ToolName = ExtractToolName(req)
		}
		if identity != nil {
			auditEntry.ClientID = identity.ClientID
			auditEntry.SessionID = identity.SessionID
		}

		// Apply RBAC for this item.
		if p.rbac != nil {
			_, err := p.rbac.Process(ctx, req, identity)
			if err != nil {
				// Denied — build a JSON-RPC error response for this item.
				auditEntry.Allowed = false
				auditEntry.DeniedReason = "rbac: " + err.Error()
				auditEntry.Latency = time.Since(itemStart)
				p.logAudit(auditEntry)

				errResp := MCPResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &RPCError{
						Code:    ErrCodeForbidden,
						Message: "Forbidden: " + err.Error(),
					},
				}
				raw, _ := json.Marshal(errResp)
				responses[i] = raw
				continue
			}
		}

		// Allowed — forward this item to upstream individually.
		itemBodyBytes, err := json.Marshal(req)
		if err != nil {
			// Marshal failure: treat as internal error.
			auditEntry.Allowed = false
			auditEntry.DeniedReason = "internal: marshal error"
			auditEntry.Latency = time.Since(itemStart)
			p.logAudit(auditEntry)
			errResp := MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &RPCError{Code: ErrCodeInternalError, Message: "internal error"},
			}
			raw, _ := json.Marshal(errResp)
			responses[i] = raw
			continue
		}

		// Build a per-item request with the item body.
		itemReq := r.Clone(context.WithValue(ctx, MCPRequestKey, req))
		itemReq.Body = io.NopCloser(bytes.NewReader(itemBodyBytes))
		itemReq.ContentLength = int64(len(itemBodyBytes))

		captured := &responseCapture{header: make(http.Header)}
		p.upstream.ServeHTTP(captured, itemReq)

		// If the method is tools/list, apply RBAC filtering on the response.
		if req.Method == MethodToolsList && p.rbac != nil && identity != nil {
			filtered, filterErr := p.filterToolsListResponse(captured.body.Bytes(), identity)
			if filterErr == nil {
				responses[i] = filtered
			} else {
				responses[i] = captured.body.Bytes()
			}
		} else {
			responses[i] = captured.body.Bytes()
		}

		// Trim trailing newline from JSON encoder output if present.
		responses[i] = bytes.TrimRight(responses[i], "\n")

		auditEntry.Allowed = true
		auditEntry.Latency = time.Since(itemStart)
		p.logAudit(auditEntry)
	}

	// Write the batch response array.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	result, _ := json.Marshal(responses)
	w.Write(result) //nolint:errcheck
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
