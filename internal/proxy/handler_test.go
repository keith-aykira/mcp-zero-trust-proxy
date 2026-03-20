package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
)

// makeTestConfig returns a minimal config pointing at the given upstream URL.
func makeTestConfig(upstreamURL string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			UpstreamURL: upstreamURL,
			ListenAddr:  ":8080",
		},
	}
}

// TestHandler_POST_ProxiesJSONRPC verifies POST with JSON-RPC body is forwarded and response returned.
func TestHandler_POST_ProxiesJSONRPC(t *testing.T) {
	// Set up fake upstream that echoes a JSON-RPC response.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Verify the request body was forwarded.
		if !strings.Contains(string(body), "tools/list") {
			t.Errorf("upstream did not receive expected body, got: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{"tools":[]},"id":1}`)
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	body := `{"jsonrpc":"2.0","method":"tools/list","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "tools") {
		t.Errorf("response missing expected content, got: %s", rr.Body.String())
	}
}

// TestHandler_SSE_OpensEventStream verifies GET with Accept: text/event-stream opens SSE connection.
func TestHandler_SSE_OpensEventStream(t *testing.T) {
	// Set up fake upstream SSE server.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("upstream: ResponseWriter does not implement Flusher")
			return
		}
		fmt.Fprintf(w, "data: {\"type\":\"ping\"}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), "data:") {
		t.Errorf("expected SSE data in response, got: %s", rr.Body.String())
	}
}

// TestHandler_SSE_StreamsWithoutBuffering verifies SSE events are streamed to client.
func TestHandler_SSE_StreamsWithoutBuffering(t *testing.T) {
	events := []string{
		"data: {\"seq\":1}\n\n",
		"data: {\"seq\":2}\n\n",
		"data: {\"seq\":3}\n\n",
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, ev := range events {
			fmt.Fprint(w, ev)
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/event-stream")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	for i, ev := range events {
		// Strip newlines for comparison purposes
		evData := strings.TrimSpace(strings.Split(ev, "\n")[0])
		if !strings.Contains(body, evData) {
			t.Errorf("event %d not found in response. Want %q in: %s", i+1, evData, body)
		}
	}
}

// TestHandler_ParsedMCPRequest_InContext verifies handler stores parsed MCPRequest in context.
func TestHandler_ParsedMCPRequest_InContext(t *testing.T) {
	var capturedCtx context.Context

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{},"id":1}`)
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"bash"},"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if capturedCtx == nil {
		t.Fatal("upstream did not capture context")
	}
	// The context should contain the parsed MCP request.
	mcpReq, ok := capturedCtx.Value(MCPRequestKey).(*MCPRequest)
	if !ok || mcpReq == nil {
		t.Error("expected MCPRequest in context, got nil or wrong type")
		return
	}
	if mcpReq.Method != MethodToolsCall {
		t.Errorf("expected method %q in context, got %q", MethodToolsCall, mcpReq.Method)
	}
}

// TestHandler_Upstream500_ReturnsError verifies upstream 500 results in error response.
func TestHandler_Upstream500_ReturnsError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	body := `{"jsonrpc":"2.0","method":"tools/list","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Upstream 500 should be returned to client (proxy passes through status codes).
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// TestHandler_UpstreamTimeout_ReturnsError verifies context deadline cancellation is handled.
func TestHandler_UpstreamTimeout_ReturnsError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow upstream by sleeping longer than client timeout.
		select {
		case <-r.Context().Done():
			// Client cancelled
		case <-time.After(5 * time.Second):
			fmt.Fprintln(w, `{"jsonrpc":"2.0","result":{},"id":1}`)
		}
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	body := `{"jsonrpc":"2.0","method":"tools/list","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	// Should return some error response, not hang.
	if rr.Code == http.StatusOK {
		t.Errorf("expected non-200 for timeout, got 200; body: %s", rr.Body.String())
	}
}

// TestHandler_NonJSONRPC_ForwardedTransparently verifies non-JSON-RPC POSTs pass through.
func TestHandler_NonJSONRPC_ForwardedTransparently(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Echo body back.
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	// Non-JSON body (not JSON-RPC).
	rawBody := "not json rpc at all"
	req := httptest.NewRequest(http.MethodPost, "/health", strings.NewReader(rawBody))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for transparent proxy, got %d", rr.Code)
	}
	if rr.Body.String() != rawBody {
		t.Errorf("expected echoed body %q, got %q", rawBody, rr.Body.String())
	}
}

// TestProxySSE_StreamsEvents is a focused test of the ProxySSE function.
func TestProxySSE_StreamsEvents(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: hello\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: world\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()

	// Check that both events arrived.
	scanner := bufio.NewScanner(strings.NewReader(body))
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if len(dataLines) < 2 {
		t.Errorf("expected at least 2 SSE data lines, got %d; body: %s", len(dataLines), body)
	}
}

// TestHandler_BodyAvailableForBothParseAndProxy ensures body is not consumed before forwarding.
func TestHandler_BodyAvailableForBothParseAndProxy(t *testing.T) {
	var receivedBody []byte

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("upstream read body error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		// Return minimal valid JSON-RPC response.
		resp := MCPResponse{JSONRPC: "2.0", ID: 1}
		json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	originalBody := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"read_file","arguments":{"path":"/tmp/test"}},"id":7}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(originalBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if string(receivedBody) != originalBody {
		t.Errorf("upstream received modified body.\nWant: %s\n Got: %s", originalBody, receivedBody)
	}
}

// TestNewHandler_InvalidUpstreamURL verifies NewHandler returns error for invalid URL.
func TestNewHandler_InvalidUpstreamURL(t *testing.T) {
	cfg := makeTestConfig("://invalid url")
	_, err := NewHandler(cfg)
	if err == nil {
		t.Fatal("expected error for invalid upstream URL, got nil")
	}
}

// TestNewHandler_EmptyUpstreamURL verifies NewHandler returns error for empty URL.
func TestNewHandler_EmptyUpstreamURL(t *testing.T) {
	cfg := makeTestConfig("")
	_, err := NewHandler(cfg)
	if err == nil {
		t.Fatal("expected error for empty upstream URL, got nil")
	}
}

// TestHandler_BodyReadAndForwarded verifies body is forwarded to upstream.
func TestHandler_BodyReadAndForwarded(t *testing.T) {
	var upstreamBody bytes.Buffer
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(&upstreamBody, r.Body)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{},"id":1}`)
	}))
	defer upstream.Close()

	cfg := makeTestConfig(upstream.URL)
	h, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler error: %v", err)
	}

	reqBody := `{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"file:///data.json"},"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if upstreamBody.String() != reqBody {
		t.Errorf("upstream received wrong body.\nWant: %s\n Got: %s", reqBody, upstreamBody.String())
	}
}
