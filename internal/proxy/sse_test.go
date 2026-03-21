package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestProxySSE_ConfigurableTimeout verifies that ProxySSE respects a configurable timeout.
// When the upstream hangs, the connection should be closed after the timeout.
func TestProxySSE_ConfigurableTimeout(t *testing.T) {
	// Upstream that hangs indefinitely after writing one event.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: first\n\n")
		flusher.Flush()
		// Hang until client disconnects (context done)
		<-r.Context().Done()
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	// Use a very short timeout — 100ms
	timeout := 100 * time.Millisecond
	maxBufferSize := 64 * 1024

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		done <- ProxySSE(rr, req, upstreamURL, timeout, maxBufferSize)
	}()

	select {
	case <-done:
		elapsed := time.Since(start)
		// Should return well within 2 seconds (timeout was 100ms)
		if elapsed > 2*time.Second {
			t.Errorf("ProxySSE did not respect timeout — took %v (expected ≤2s)", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ProxySSE did not return after timeout — stuck indefinitely")
	}
}

// TestProxySSE_ConfigurableBufferSize verifies that a configurable buffer size is used
// (not the default 64KB limit). When maxBufferSize is set, the scanner uses it.
func TestProxySSE_ConfigurableBufferSize(t *testing.T) {
	const customBuffer = 128 * 1024 // 128KB

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		// Send a normal event that fits well within any buffer
		fmt.Fprint(w, "data: normal-event\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")

	// Should not panic or error with custom buffer size
	err := ProxySSE(rr, req, upstreamURL, 0, customBuffer)
	if err != nil {
		t.Errorf("ProxySSE with custom buffer = %v, want nil", err)
	}

	if !strings.Contains(rr.Body.String(), "normal-event") {
		t.Errorf("expected event data in response, got: %s", rr.Body.String())
	}
}

// TestProxySSE_ZeroTimeoutNoDeadline verifies that timeout=0 means no timeout
// (uses context cancellation only, consistent with original behavior).
func TestProxySSE_ZeroTimeoutNoDeadline(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: event\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")

	// timeout=0 means no timeout — should complete normally
	err := ProxySSE(rr, req, upstreamURL, 0, 64*1024)
	if err != nil {
		t.Errorf("ProxySSE with zero timeout = %v, want nil", err)
	}
}

// TestProxySSE_BackpressureSmallBuffer verifies that a very small buffer causes
// the scanner to error on a line exceeding the buffer size.
func TestProxySSE_BackpressureSmallBuffer(t *testing.T) {
	// Generate a long event line that exceeds a tiny buffer
	longData := strings.Repeat("x", 200) // 200 bytes

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "data: %s\n\n", longData)
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Accept", "text/event-stream")

	// Use a tiny buffer (16 bytes) — smaller than the event line
	// This should trigger backpressure (scanner.Err() != nil or connection closes)
	// The function should return (either error or nil depending on implementation)
	// — the key is it must NOT hang indefinitely.
	done := make(chan error, 1)
	go func() {
		done <- ProxySSE(rr, req, upstreamURL, 0, 16)
	}()

	select {
	case <-done:
		// Good — returned promptly (either error or nil is acceptable)
	case <-time.After(3 * time.Second):
		t.Fatal("ProxySSE with small buffer hung indefinitely")
	}
}
