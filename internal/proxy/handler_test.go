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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
)

// makeTestConfig returns a minimal config pointing at the given upstream URL.
func makeTestConfig(upstreamURL string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			ListenAddr: ":8080",
			Registry: config.ServerRegistryConfig{
				Default: "default",
				Servers: []config.UpstreamServerConfig{
					{
						Name:    "default",
						URL:     upstreamURL,
						Enabled: true,
					},
				},
			},
		},
	}
}

// makeTestRouter creates a new server router for testing.
func makeTestRouter(upstreamURL string) (Router, error) {
	cfg := makeTestConfig(upstreamURL)
	return NewServerRouter(&cfg.Server.Registry)
}

// TestRouter is a minimal router implementation for testing that always returns
// a "default" server. Use makeDefaultRouter to create one.
type TestRouter struct {
	upstream *url.URL
}

func (r *TestRouter) ResolveForRequest(req *http.Request) (*url.URL, string, error) {
	return r.upstream, "default", nil
}

func (r *TestRouter) GetServerUpstream(name string) (*url.URL, error) {
	return r.upstream, nil
}

func (r *TestRouter) GetServerTimeout(name string) int { return 120 }

func (r *TestRouter) GetServerNames() []string { return []string{"default"} }

func (r *TestRouter) RefreshToolsCache() error { return nil }

func (r *TestRouter) GetAggregatedTools() []config.ToolInfo { return nil }

// makeDefaultRouter creates a test router that routes everything to a "default" server
func makeDefaultRouter(upstreamURL string) (*url.URL, Router, error) {
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, nil, err
	}
	return upstream, &TestRouter{upstream: upstream}, nil
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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

// contextCapturingTransport is a custom RoundTripper that captures the outgoing
// request context so tests can inspect values stored by ServeHTTP.
type contextCapturingTransport struct {
	wrapped    http.RoundTripper
	capturedCh chan context.Context
}

func (t *contextCapturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case t.capturedCh <- req.Context():
	default:
	}
	return t.wrapped.RoundTrip(req)
}

// TestHandler_DoesNotParseBody_InContext verifies that Handler does NOT set MCPRequestKey in context.
// Body parsing and context injection are now exclusively handled by Pipeline (HARD-11).
// We capture the outgoing request context via a custom RoundTripper to confirm no MCPRequestKey.
func TestHandler_DoesNotParseBody_InContext(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{},"id":1}`)
	}))
	defer upstream.Close()

        cfg := makeTestConfig(upstream.URL)
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
        if err != nil {
                t.Fatalf("NewHandler error: %v", err)
        }

	// Inject a context-capturing transport so we can see what context was forwarded.
	capturedCh := make(chan context.Context, 1)
	transport := &contextCapturingTransport{
		wrapped:    http.DefaultTransport,
		capturedCh: capturedCh,
	}
	h.SetTransport(transport)

	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"bash"},"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	select {
	case capturedCtx := <-capturedCh:
		// Handler must NOT set MCPRequestKey — Pipeline is responsible for this
		mcpReq := capturedCtx.Value(MCPRequestKey)
		if mcpReq != nil {
			t.Errorf("Handler should NOT set MCPRequestKey in context — Pipeline handles this; got %v", mcpReq)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for transport to capture context")
	}
}

// TestHandler_Upstream500_ReturnsError verifies upstream 500 results in error response.
func TestHandler_Upstream500_ReturnsError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer upstream.Close()

        cfg := makeTestConfig(upstream.URL)
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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

// TestNewHandler_NilRouter verifies NewHandler returns error when router is nil.
func TestNewHandler_NilRouter(t *testing.T) {
	_, err := NewHandler(&config.Config{}, nil)
	if err == nil {
		t.Fatal("expected error for nil router, got nil")
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
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
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

// TestHandler_MultipleUpstreams tests routing to different upstreams.
func TestHandler_MultipleUpstreams(t *testing.T) {
        upstream1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                fmt.Fprint(w, `"server1"`)
        }))
        defer upstream1.Close()

        upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                fmt.Fprint(w, `"server2"`)
        }))
        defer upstream2.Close()

        cfg := &config.Config{
                Server: config.ServerConfig{
                        Registry: config.ServerRegistryConfig{
                                Default: "server1",
                                Servers: []config.UpstreamServerConfig{
                                        {Name: "server1", URL: upstream1.URL, Enabled: true},
                                        {Name: "server2", URL: upstream2.URL, Enabled: true},
                                },
                        },
                },
        }

        router, err := NewServerRouter(&cfg.Server.Registry)
        if err != nil {
                t.Fatalf("NewServerRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
        if err != nil {
                t.Fatalf("NewHandler error: %v", err)
        }

        // Test server1 (default)
        req1 := httptest.NewRequest(http.MethodGet, "/", nil)
        rr1 := httptest.NewRecorder()
        h.ServeHTTP(rr1, req1)
        if rr1.Body.String() != `"server1"` {
                t.Errorf("expected server1 response, got %s", rr1.Body.String())
        }

        // Test server2 via path
        req2 := httptest.NewRequest(http.MethodGet, "/server2/", nil)
        rr2 := httptest.NewRecorder()
        h.ServeHTTP(rr2, req2)
        if rr2.Body.String() != `"server2"` {
                t.Errorf("expected server2 response, got %s", rr2.Body.String())
        }
}

// TestHandler_UnknownServer tests that non-existent server names fall back to default or return 404.
func TestHandler_UnknownServer(t *testing.T) {
        // Case 1: Unknown server with default configured -> should use default (200)
        upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                fmt.Fprint(w, `"default_response"`)
        }))
        defer upstream.Close()

        cfg := &config.Config{
                Server: config.ServerConfig{
                        Registry: config.ServerRegistryConfig{
                                Default: "default",
                                Servers: []config.UpstreamServerConfig{
                                        {Name: "default", URL: upstream.URL, Enabled: true},
                                },
                        },
                },
        }

        router, err := NewServerRouter(&cfg.Server.Registry)
        if err != nil {
                t.Fatalf("NewServerRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
        if err != nil {
                t.Fatalf("NewHandler error: %v", err)
        }

        // Unknown server name falls back to default
        req := httptest.NewRequest(http.MethodGet, "/unknown_server/", nil)
        rr := httptest.NewRecorder()
        h.ServeHTTP(rr, req)

        // Should fall back to default server (200)
        if rr.Code != http.StatusOK {
                t.Errorf("expected 200 (fallback to default), got %d", rr.Code)
        }

        // Case 2: No default server and unknown server name -> 404
        cfg2 := &config.Config{
                Server: config.ServerConfig{
                        Registry: config.ServerRegistryConfig{
                                Default: "", // No default
                                Servers: []config.UpstreamServerConfig{
                                        {Name: "onlyserver", URL: upstream.URL, Enabled: true},
                                },
                        },
                },
        }

        router2, err := NewServerRouter(&cfg2.Server.Registry)
        if err != nil {
                t.Fatalf("NewServerRouter error: %v", err)
        }
        h2, err := NewHandler(cfg2, router2)
        if err != nil {
                t.Fatalf("NewHandler error: %v", err)
        }

        req2 := httptest.NewRequest(http.MethodGet, "/unknown_server/", nil)
        rr2 := httptest.NewRecorder()
        h2.ServeHTTP(rr2, req2)

        // Should return 404 since no default configured
        if rr2.Code != http.StatusNotFound {
                t.Errorf("expected 404 for unknown server with no default, got %d", rr2.Code)
        }
}

// TestHandler_SSEWithQualityValue tests SSE detection with quality values in Accept header.
func TestHandler_SSEWithQualityValue(t *testing.T) {
        upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "text/event-stream")
                w.WriteHeader(http.StatusOK)
                fmt.Fprint(w, "data: test\n\n")
        }))
        defer upstream.Close()

        cfg := makeTestConfig(upstream.URL)
        router, err := makeTestRouter(upstream.URL)
        if err != nil {
                t.Fatalf("makeTestRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
        if err != nil {
                t.Fatalf("NewHandler error: %v", err)
        }

        req := httptest.NewRequest(http.MethodGet, "/sse", nil)
        req.Header.Set("Accept", "text/event-stream, application/json;q=0.9")
        rr := httptest.NewRecorder()
        h.ServeHTTP(rr, req)

        ct := rr.Header().Get("Content-Type")
        if !strings.Contains(ct, "text/event-stream") {
                t.Errorf("expected SSE content type, got %q", ct)
        }
}

// TestHandler_SetTransport sets custom transport and uses it.
func TestHandler_SetTransport(t *testing.T) {
        customTransportCalled := false
        customTransport := &testTransport{
                roundTrip: func(r *http.Request) (*http.Response, error) {
                        customTransportCalled = true
                        return &http.Response{
                                StatusCode: http.StatusOK,
                                Body:       io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{}}`)),
                        }, nil
                },
        }

        cfg := makeTestConfig("http://unused")
        router, err := NewServerRouter(&cfg.Server.Registry)
        if err != nil {
                t.Fatalf("NewServerRouter error: %v", err)
        }
        h, err := NewHandler(cfg, router)
        if err != nil {
                t.Fatalf("NewHandler error: %v", err)
        }
        h.SetTransport(customTransport)

        req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test"}`))
        rr := httptest.NewRecorder()
        h.ServeHTTP(rr, req)

        if !customTransportCalled {
                t.Error("custom transport was not called")
        }
}

// testTransport is a simple http.RoundTripper for testing.
type testTransport struct {
        roundTrip func(*http.Request) (*http.Response, error)
}

func (t *testTransport) RoundTrip(r *http.Request) (*http.Response, error) {
        return t.roundTrip(r)
}
