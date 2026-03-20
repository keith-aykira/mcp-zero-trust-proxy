package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestParseRequest_ToolsCall verifies tools/call extracts method and tool name from params.
func TestParseRequest_ToolsCall(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"bash","arguments":{"cmd":"ls"}},"id":1}`)
	reqs, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	req := reqs[0]
	if req.Method != MethodToolsCall {
		t.Errorf("expected method %q, got %q", MethodToolsCall, req.Method)
	}
	toolName := ExtractToolName(req)
	if toolName != "bash" {
		t.Errorf("expected tool name %q, got %q", "bash", toolName)
	}
}

// TestParseRequest_ToolsList verifies tools/list extracts method.
func TestParseRequest_ToolsList(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"tools/list","id":2}`)
	reqs, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Method != MethodToolsList {
		t.Errorf("expected method %q, got %q", MethodToolsList, reqs[0].Method)
	}
}

// TestParseRequest_Initialize verifies initialize extracts method.
func TestParseRequest_Initialize(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"initialize","params":{"capabilities":{}},"id":1}`)
	reqs, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Method != MethodInitialize {
		t.Errorf("expected method %q, got %q", MethodInitialize, reqs[0].Method)
	}
}

// TestParseRequest_ResourcesRead verifies resources/read extracts method and resource URI.
func TestParseRequest_ResourcesRead(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"file:///foo/bar.txt"},"id":3}`)
	reqs, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Method != MethodResourcesRead {
		t.Errorf("expected method %q, got %q", MethodResourcesRead, reqs[0].Method)
	}
	uri := ExtractResourceURI(reqs[0])
	if uri != "file:///foo/bar.txt" {
		t.Errorf("expected URI %q, got %q", "file:///foo/bar.txt", uri)
	}
}

// TestParseRequest_PromptsGet verifies prompts/get extracts method and prompt name.
func TestParseRequest_PromptsGet(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"prompts/get","params":{"name":"summary-prompt"},"id":4}`)
	reqs, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Method != MethodPromptsGet {
		t.Errorf("expected method %q, got %q", MethodPromptsGet, reqs[0].Method)
	}
}

// TestParseRequest_Batch verifies batch JSON-RPC requests are parsed into individual requests.
func TestParseRequest_Batch(t *testing.T) {
	body := []byte(`[{"jsonrpc":"2.0","method":"tools/list","id":1},{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read_file"},"id":2}]`)
	reqs, err := ParseRequest(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}
	if reqs[0].Method != MethodToolsList {
		t.Errorf("expected first method %q, got %q", MethodToolsList, reqs[0].Method)
	}
	if reqs[1].Method != MethodToolsCall {
		t.Errorf("expected second method %q, got %q", MethodToolsCall, reqs[1].Method)
	}
}

// TestParseRequest_MalformedJSON verifies malformed JSON returns error (not panic).
func TestParseRequest_MalformedJSON(t *testing.T) {
	body := []byte(`{not valid json`)
	_, err := ParseRequest(body)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestParseRequest_MissingMethod verifies missing "method" field returns error.
func TestParseRequest_MissingMethod(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1}`)
	_, err := ParseRequest(body)
	if err == nil {
		t.Fatal("expected error for missing method field, got nil")
	}
}

// TestWriteError verifies WriteError produces valid JSON-RPC error response with code and message.
func TestWriteError(t *testing.T) {
	var buf bytes.Buffer
	err := WriteError(&buf, 42, ErrCodeMethodNotFound, "method not found")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp MCPResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("WriteError output is not valid JSON: %v, output: %s", err, buf.String())
	}
	if resp.Error == nil {
		t.Fatal("expected error field in response")
	}
	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("expected error code %d, got %d", ErrCodeMethodNotFound, resp.Error.Code)
	}
	if resp.Error.Message != "method not found" {
		t.Errorf("expected error message %q, got %q", "method not found", resp.Error.Message)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc %q, got %q", "2.0", resp.JSONRPC)
	}
}

// TestWriteResponse verifies WriteResponse produces valid JSON-RPC success response with result.
func TestWriteResponse(t *testing.T) {
	var buf bytes.Buffer
	result := map[string]string{"tools": "list"}
	err := WriteResponse(&buf, 99, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp MCPResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("WriteResponse output is not valid JSON: %v, output: %s", err, buf.String())
	}
	if resp.Error != nil {
		t.Errorf("expected no error field, got: %+v", resp.Error)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc %q, got %q", "2.0", resp.JSONRPC)
	}
	// Check the result contains expected data
	if !strings.Contains(string(resp.Result), "list") {
		t.Errorf("expected result to contain 'list', got: %s", resp.Result)
	}
}
