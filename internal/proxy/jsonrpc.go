package proxy

import (
	"encoding/json"
	"fmt"
	"io"
)

// MCP method constants define the standard JSON-RPC method names used in MCP protocol.
const (
	MethodInitialize    = "initialize"
	MethodToolsList     = "tools/list"
	MethodToolsCall     = "tools/call"
	MethodResourcesRead = "resources/read"
	MethodPromptsGet    = "prompts/get"
)

// Standard JSON-RPC 2.0 error codes plus MCP-specific custom codes.
const (
	// Standard JSON-RPC 2.0 error codes.
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603

	// MCP-specific custom error codes.
	ErrCodeUnauthorized = -32001
	ErrCodeForbidden    = -32002
	ErrCodeRateLimited  = -32003
)

// toolsCallParams is used to extract the tool name from tools/call requests.
type toolsCallParams struct {
	Name string `json:"name"`
}

// resourcesReadParams is used to extract the URI from resources/read requests.
type resourcesReadParams struct {
	URI string `json:"uri"`
}

// ParseRequest parses a raw JSON body as either a single or batch JSON-RPC 2.0 request.
// It handles both object (single) and array (batch) forms.
// Returns an error for malformed JSON or a request with a missing method field.
func ParseRequest(body []byte) ([]*MCPRequest, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty request body")
	}

	// Detect batch vs single by looking at the first non-whitespace byte.
	trimmed := trimLeftSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty request body")
	}

	if trimmed[0] == '[' {
		// Batch request: array of JSON-RPC objects.
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("parse error: %w", err)
		}
		reqs := make([]*MCPRequest, 0, len(batch))
		for i, raw := range batch {
			req, err := parseSingle(raw)
			if err != nil {
				return nil, fmt.Errorf("parse error in batch item %d: %w", i, err)
			}
			reqs = append(reqs, req)
		}
		return reqs, nil
	}

	// Single request.
	req, err := parseSingle(body)
	if err != nil {
		return nil, err
	}
	return []*MCPRequest{req}, nil
}

// parseSingle parses a single JSON-RPC 2.0 object.
func parseSingle(body []byte) (*MCPRequest, error) {
	var req MCPRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if req.Method == "" {
		return nil, fmt.Errorf("invalid request: missing method field")
	}
	return &req, nil
}

// trimLeftSpace returns a slice of b with leading ASCII whitespace removed.
// Avoids allocation by returning a sub-slice.
func trimLeftSpace(b []byte) []byte {
	for i, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return b[i:]
		}
	}
	return b[len(b):]
}

// ExtractToolName returns the tool name from a tools/call request's params.
// Returns an empty string for any other method or if params are absent/malformed.
func ExtractToolName(req *MCPRequest) string {
	if req == nil || req.Method != MethodToolsCall {
		return ""
	}
	if len(req.Params) == 0 {
		return ""
	}
	var p toolsCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return ""
	}
	return p.Name
}

// ExtractResourceURI returns the resource URI from a resources/read request's params.
// Returns an empty string for any other method or if params are absent/malformed.
func ExtractResourceURI(req *MCPRequest) string {
	if req == nil || req.Method != MethodResourcesRead {
		return ""
	}
	if len(req.Params) == 0 {
		return ""
	}
	var p resourcesReadParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return ""
	}
	return p.URI
}

// WriteError writes a JSON-RPC 2.0 error response to w.
// id may be nil (for parse errors before ID is known), a number, or a string.
func WriteError(w io.Writer, id interface{}, code int, message string) error {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	return json.NewEncoder(w).Encode(resp)
}

// WriteResponse writes a JSON-RPC 2.0 success response to w.
// result is JSON-serialized and placed in the "result" field.
func WriteResponse(w io.Writer, id interface{}, result interface{}) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage(raw),
	}
	return json.NewEncoder(w).Encode(resp)
}
