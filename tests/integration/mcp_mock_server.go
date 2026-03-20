package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

// ServerType identifies which MCP server behavior to simulate.
type ServerType int

const (
	// ToolsOnlyServer simulates a typical tool-serving MCP server.
	// Handles: initialize, tools/list (5 tools), tools/call
	ToolsOnlyServer ServerType = iota

	// ResourcesServer simulates a resource-serving MCP server.
	// Handles: initialize, resources/list, resources/read
	ResourcesServer

	// PromptsServer simulates a prompt-serving MCP server.
	// Handles: initialize, prompts/list, prompts/get
	PromptsServer

	// MixedServer simulates a full-featured MCP server.
	// Handles: all MCP methods (tools + resources + prompts)
	MixedServer

	// SSEStreamingServer simulates a server with Server-Sent Events streaming.
	// Handles: initialize, tools/call with SSE streaming response
	SSEStreamingServer
)

// jsonRPCRequest is the incoming JSON-RPC 2.0 request shape.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// jsonRPCResponse is the outgoing JSON-RPC 2.0 response shape.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolCallParams is used to extract the tool name from a tools/call request.
type toolCallParams struct {
	Name string `json:"name"`
}

// MockServer is a configurable HTTP test server simulating an MCP server.
type MockServer struct {
	Server     *httptest.Server
	serverType ServerType
}

// NewMockServer creates a new mock MCP server of the given type.
// The returned MockServer's Server field is ready to receive requests.
func NewMockServer(st ServerType) *MockServer {
	m := &MockServer{serverType: st}
	switch st {
	case SSEStreamingServer:
		m.Server = httptest.NewServer(http.HandlerFunc(m.handleSSE))
	default:
		m.Server = httptest.NewServer(http.HandlerFunc(m.handleJSONRPC))
	}
	return m
}

// Close shuts down the mock server.
func (m *MockServer) Close() {
	m.Server.Close()
}

// URL returns the mock server's base URL.
func (m *MockServer) URL() string {
	return m.Server.URL
}

// handleJSONRPC is the main JSON-RPC handler for non-streaming server types.
func (m *MockServer) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(jsonRPCResponse{ //nolint:errcheck
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "Parse error"},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := m.dispatch(req)
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// dispatch routes an incoming JSON-RPC request to the appropriate handler based on server type.
func (m *MockServer) dispatch(req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return m.handleInitialize(req)
	case "tools/list":
		if m.serverType == ToolsOnlyServer || m.serverType == MixedServer || m.serverType == ResourcesServer || m.serverType == PromptsServer {
			return m.handleToolsList(req)
		}
	case "tools/call":
		if m.serverType == ToolsOnlyServer || m.serverType == MixedServer {
			return m.handleToolsCall(req)
		}
	case "resources/list":
		if m.serverType == ResourcesServer || m.serverType == MixedServer {
			return m.handleResourcesList(req)
		}
	case "resources/read":
		if m.serverType == ResourcesServer || m.serverType == MixedServer {
			return m.handleResourcesRead(req)
		}
	case "prompts/list":
		if m.serverType == PromptsServer || m.serverType == MixedServer {
			return m.handlePromptsList(req)
		}
	case "prompts/get":
		if m.serverType == PromptsServer || m.serverType == MixedServer {
			return m.handlePromptsGet(req)
		}
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error:   &rpcError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
	}
}

// handleInitialize responds to the MCP initialize handshake.
func (m *MockServer) handleInitialize(req jsonRPCRequest) jsonRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "MockMCPServer",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{
			"tools":     map[string]bool{"listChanged": false},
			"resources": map[string]bool{"listChanged": false},
			"prompts":   map[string]bool{"listChanged": false},
		},
	}
	raw, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(raw)}
}

// handleToolsList returns 5 test tools.
func (m *MockServer) handleToolsList(req jsonRPCRequest) jsonRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        "read_file",
			"description": "Read the contents of a file",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]string{"type": "string", "description": "File path"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "write_file",
			"description": "Write content to a file",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]string{"type": "string"},
					"content": map[string]string{"type": "string"},
				},
			},
		},
		{
			"name":        "list_dir",
			"description": "List directory contents",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]string{"type": "string"},
				},
			},
		},
		{
			"name":        "search",
			"description": "Search for text in files",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]string{"type": "string"},
					"path":  map[string]string{"type": "string"},
				},
			},
		},
		{
			"name":        "execute_command",
			"description": "Execute a shell command",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]string{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	result := map[string]interface{}{"tools": tools}
	raw, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(raw)}
}

// handleToolsCall returns a successful tool execution result.
func (m *MockServer) handleToolsCall(req jsonRPCRequest) jsonRPCResponse {
	var params toolCallParams
	if len(req.Params) > 0 {
		json.Unmarshal(req.Params, &params) //nolint:errcheck
	}
	result := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Tool '%s' executed successfully", params.Name),
			},
		},
		"isError": false,
	}
	raw, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(raw)}
}

// handleResourcesList returns a list of file-like resources.
func (m *MockServer) handleResourcesList(req jsonRPCRequest) jsonRPCResponse {
	resources := []map[string]interface{}{
		{
			"uri":      "file:///project/README.md",
			"name":     "README",
			"mimeType": "text/markdown",
		},
		{
			"uri":      "file:///project/main.go",
			"name":     "main.go",
			"mimeType": "text/x-go",
		},
		{
			"uri":      "file:///project/config.yaml",
			"name":     "config.yaml",
			"mimeType": "application/yaml",
		},
	}
	result := map[string]interface{}{"resources": resources}
	raw, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(raw)}
}

// handleResourcesRead returns the content of a resource.
func (m *MockServer) handleResourcesRead(req jsonRPCRequest) jsonRPCResponse {
	result := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"uri":      "file:///project/README.md",
				"mimeType": "text/markdown",
				"text":     "# Mock Project\n\nThis is a mock resource for testing.",
			},
		},
	}
	raw, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(raw)}
}

// handlePromptsList returns a list of available prompt templates.
func (m *MockServer) handlePromptsList(req jsonRPCRequest) jsonRPCResponse {
	prompts := []map[string]interface{}{
		{
			"name":        "code_review",
			"description": "Review code for issues",
			"arguments": []map[string]interface{}{
				{"name": "code", "description": "Code to review", "required": true},
			},
		},
		{
			"name":        "summarize",
			"description": "Summarize a document",
			"arguments": []map[string]interface{}{
				{"name": "document", "description": "Document to summarize", "required": true},
			},
		},
	}
	result := map[string]interface{}{"prompts": prompts}
	raw, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(raw)}
}

// handlePromptsGet returns a rendered prompt template.
func (m *MockServer) handlePromptsGet(req jsonRPCRequest) jsonRPCResponse {
	result := map[string]interface{}{
		"description": "A code review prompt",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": map[string]interface{}{
					"type": "text",
					"text": "Please review the following code:\n\n```go\nfunc main() {}\n```",
				},
			},
		},
	}
	raw, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(raw)}
}

// handleSSE handles SSE streaming requests.
// It sends 3 progress events followed by a final result event.
// When the client sends Accept: text/event-stream, the proxy delegates to ProxySSE
// which calls this handler directly. All paths are accepted.
func (m *MockServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Read body if present. For SSE connections the proxy may send the body.
	var req jsonRPCRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Body not decodable — treat as SSE tool call anyway.
			req.Method = "tools/call"
			req.ID = 1
		}
	} else {
		// No body: default to tools/call for SSE tests.
		req.Method = "tools/call"
		req.ID = 1
	}

	if req.Method == "" {
		req.Method = "tools/call"
	}

	if req.Method != "tools/call" {
		// Non-streaming methods get normal JSON-RPC response.
		w.Header().Set("Content-Type", "application/json")
		resp := m.dispatch(req)
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
		return
	}

	// SSE response for tools/call.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send 3 progress events.
	for i := 1; i <= 3; i++ {
		progressEvent := map[string]interface{}{
			"type":     "progress",
			"progress": i * 33,
			"message":  fmt.Sprintf("Processing step %d of 3", i),
		}
		progressJSON, _ := json.Marshal(progressEvent)
		fmt.Fprintf(w, "data: %s\n\n", progressJSON)
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)
	}

	// Send the final result event.
	finalResult := map[string]interface{}{
		"type": "result",
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Streaming tool execution complete",
				},
			},
			"isError": false,
		},
	}
	finalJSON, _ := json.Marshal(finalResult)
	fmt.Fprintf(w, "data: %s\n\n", finalJSON)
	flusher.Flush()
}
