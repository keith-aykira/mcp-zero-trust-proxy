package proxy

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ProxySSE establishes a Server-Sent Events connection to the upstream and streams
// events back to the client in real time. It sets all required SSE response headers
// and reads upstream events line by line, flushing each to the client immediately.
//
// Connection lifecycle: returns when either the client disconnects (r.Context() done)
// or the upstream closes the connection.
func ProxySSE(w http.ResponseWriter, r *http.Request, upstream *url.URL) error {
	// SSE requires the ResponseWriter to implement http.Flusher for immediate delivery.
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported: ResponseWriter does not implement http.Flusher")
	}

	// Set SSE response headers before writing any body.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering if present.

	// Build upstream request using the incoming request's context so that
	// client disconnection propagates as context cancellation.
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream.String(), nil)
	if err != nil {
		return fmt.Errorf("create upstream SSE request: %w", err)
	}

	// Forward original request headers (auth tokens, session IDs, etc.) to upstream.
	for key, vals := range r.Header {
		// Skip hop-by-hop headers that should not be forwarded.
		if isHopByHopHeader(key) {
			continue
		}
		for _, v := range vals {
			upstreamReq.Header.Add(key, v)
		}
	}
	upstreamReq.Header.Set("Accept", "text/event-stream")
	upstreamReq.Header.Set("Cache-Control", "no-cache")

	// Use a default client; caller may inject a custom one in future via options.
	client := &http.Client{}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return fmt.Errorf("connect to upstream SSE: %w", err)
	}
	defer resp.Body.Close()

	// Write the upstream status code and any non-hop-by-hop headers.
	for key, vals := range resp.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, v := range vals {
			w.Header().Set(key, v)
		}
	}
	// Always override Content-Type to ensure SSE headers are set.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(resp.StatusCode)
	flusher.Flush()

	// Stream events from upstream to client line by line.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(w, line)
		flusher.Flush()

		// Check if client has disconnected.
		select {
		case <-r.Context().Done():
			return nil
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		// Ignore context-cancelled errors — they are normal client disconnects.
		if strings.Contains(err.Error(), "context") {
			return nil
		}
		return fmt.Errorf("SSE stream read error: %w", err)
	}

	return nil
}

// isHopByHopHeader returns true for HTTP/1.1 hop-by-hop headers that must not be forwarded.
func isHopByHopHeader(key string) bool {
	hopByHop := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}
	return hopByHop[http.CanonicalHeaderKey(key)]
}
