---
phase: 02-mvp-build
plan: 02
subsystem: proxy
tags: [go, jsonrpc, sse, httputil, reverseproxy, mcp, streaming]

requires:
  - phase: 02-01
    provides: MCPRequest/MCPResponse/RPCError/ClientIdentity types in internal/proxy/interfaces.go; config.Config struct in internal/config/

provides:
  - JSON-RPC 2.0 parser with single and batch request support (ParseRequest)
  - Tool name extraction from tools/call params (ExtractToolName)
  - Resource URI extraction from resources/read params (ExtractResourceURI)
  - JSON-RPC error/response writers (WriteError, WriteResponse)
  - MCP method constants (initialize, tools/list, tools/call, resources/read, prompts/get)
  - Standard + custom MCP error codes (-32700 through -32003)
  - HTTP reverse proxy Handler with context injection (NewHandler, ServeHTTP)
  - SSE streaming proxy that avoids buffering (ProxySSE)
  - MCPRequestKey context key for middleware access to parsed request
  - SetTransport hook for transport injection

affects: [02-03, 02-04, 02-05, 02-06]

tech-stack:
  added:
    - "net/http/httputil.ReverseProxy — standard HTTP forwarding (stdlib)"
    - "bufio.Scanner — line-by-line SSE stream reading (stdlib)"
  patterns:
    - "io.ReadAll + bytes.NewReader pattern — read body once, make it available for both parse and proxy forward"
    - "contextKey type alias — unexported string type for context keys prevents cross-package collisions"
    - "TDD: test -> commit (RED) -> implement -> commit (GREEN) for each task"
    - "SetTransport method for ReverseProxy — testability hook for transport injection"
    - "isSSERequest on Accept header — detect SSE before forwarding to avoid buffering"

key-files:
  created:
    - internal/proxy/jsonrpc.go
    - internal/proxy/jsonrpc_test.go
    - internal/proxy/handler.go
    - internal/proxy/handler_test.go
    - internal/proxy/sse.go
  modified: []

key-decisions:
  - "Import cycle fix: removed middleware import from handler.go — proxy imports middleware which imports proxy. Handler uses interface{} placeholder fields; concrete wiring deferred to Plans 02-03 through 02-05"
  - "Context values do not cross HTTP connections — MCPRequest stored in context is for in-process middleware, not visible to upstream server. Test uses SetTransport hook to verify context at RoundTrip boundary"
  - "httputil.ReverseProxy for HTTP, custom ProxySSE for SSE — ReverseProxy buffers responses which breaks SSE streaming"
  - "ParseRequest fails on missing method field — callers must handle gracefully; non-JSON-RPC bodies are silently passed through by handler"
  - "SetTransport exported on Handler — enables test transport injection without exposing internal ReverseProxy field"

patterns-established:
  - "Context injection pattern: parse JSON-RPC in ServeHTTP, store in ctx via context.WithValue(ctx, MCPRequestKey, req), pass to ReverseProxy via r.WithContext(ctx)"
  - "SSE detection: Accept: text/event-stream header check before routing to ProxySSE"
  - "Body buffering: io.ReadAll + io.NopCloser(bytes.NewReader(bodyBytes)) — makes body readable twice"

requirements-completed: [PRXY-02]

duration: 6min
completed: 2026-03-20
---

# Phase 2 Plan 2: MCP Reverse Proxy Core Summary

**JSON-RPC 2.0 parser + HTTP/SSE reverse proxy with context-injected MCPRequest — 22 TDD tests pass, middleware hook points ready**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-20T14:02:38Z
- **Completed:** 2026-03-20T14:08:22Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- JSON-RPC 2.0 parser handles single and batch requests, extracts method/tool/resource from all 5 MCP method types, handles malformed input without panic
- HTTP reverse proxy buffers request body, parses JSON-RPC, injects `*MCPRequest` into request context for downstream middleware — all without consuming the body before forwarding
- SSE proxy streams events line-by-line with immediate flush using `bufio.Scanner` + `http.Flusher`, bypassing `httputil.ReverseProxy`'s buffering behavior

## Task Commits

Each task was committed atomically with TDD RED+GREEN commits:

1. **Task 1 RED: JSON-RPC 2.0 parser tests** - `79515c8` (test)
2. **Task 1 GREEN: JSON-RPC 2.0 parser implementation** - `60158d6` (feat)
3. **Task 2 RED: HTTP+SSE handler tests** - `db10769` (test)
4. **Task 2 GREEN: HTTP+SSE handler + SSE proxy implementation** - `4c63b7a` (feat)

_TDD tasks have separate RED (test) and GREEN (implementation) commits as required._

## Files Created/Modified

- `internal/proxy/jsonrpc.go` — ParseRequest (single+batch), ExtractToolName, ExtractResourceURI, WriteError, WriteResponse, MCP method constants, error codes
- `internal/proxy/jsonrpc_test.go` — 10 tests covering all 5 MCP methods, batch, malformed JSON, missing method, WriteError, WriteResponse
- `internal/proxy/handler.go` — Handler struct, NewHandler, ServeHTTP (body buffer + JSON-RPC parse + context inject + SSE routing), SetTransport, isSSERequest
- `internal/proxy/handler_test.go` — 12 tests covering POST proxy, SSE open, SSE streaming, MCPRequest in context, upstream 500, timeout, non-JSON-RPC passthrough, body forwarding
- `internal/proxy/sse.go` — ProxySSE with hop-by-hop header filtering, line-by-line Scanner streaming, client disconnect handling via context cancellation

## Decisions Made

- **Import cycle fix:** `handler.go` cannot import `internal/middleware` because `internal/middleware` already imports `internal/proxy` (for MCPRequest/ClientIdentity types). Handler middleware fields deferred to Plans 02-03 to 02-05 when the dependency structure is resolved. No functional impact — those fields are placeholders.
- **SetTransport exported:** `Handler.SetTransport(http.RoundTripper)` added to support test injection of a custom `RoundTripper`. This is the standard Go testability pattern for HTTP clients.
- **Context values don't cross HTTP:** MCPRequest stored in `r.Context()` is only visible to in-process middleware (RoundTripper boundary), not to the upstream server. Test updated to capture context at RoundTrip time via a custom transport.
- **SSE: custom vs ReverseProxy:** `httputil.ReverseProxy` buffers the full response before writing to client (except for special streaming detection logic). `ProxySSE` implements line-by-line streaming with `bufio.Scanner` + `http.Flusher.Flush()` to guarantee real-time delivery.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed import cycle between proxy and middleware packages**
- **Found during:** Task 2 (HTTP+SSE handler implementation)
- **Issue:** `handler.go` imported `internal/middleware` for Middleware/Authenticator/RateLimiter/AuditLogger interface types. But `internal/middleware/interfaces.go` already imports `internal/proxy` for MCPRequest and ClientIdentity types. Go rejects this circular dependency.
- **Fix:** Removed the `middleware` import from `handler.go`. The Handler struct now has no middleware fields until Plans 02-03 through 02-05 establish the correct dependency structure. Added a comment explaining the deferral.
- **Files modified:** `internal/proxy/handler.go`
- **Verification:** `go build ./...` succeeds, all 22 tests pass
- **Committed in:** `4c63b7a` (Task 2 GREEN commit)

**2. [Rule 1 - Bug] Fixed context test — MCPRequest context values don't cross HTTP to upstream**
- **Found during:** Task 2 (TestHandler_ParsedMCPRequest_InContext)
- **Issue:** Test captured `r.Context()` inside the upstream httptest.Server handler. But the upstream receives a NEW HTTP request (network call) — its context is the server-side context, not the client-side context where MCPRequestKey was stored.
- **Fix:** Added `SetTransport` method to Handler. Test injects a `contextCapturingTransport` (custom `RoundTripper`) that captures the outgoing request context before the network call. This is where MCPRequestKey is actually visible — at the proxy layer before forwarding.
- **Files modified:** `internal/proxy/handler.go`, `internal/proxy/handler_test.go`
- **Verification:** `TestHandler_ParsedMCPRequest_InContext` passes
- **Committed in:** `4c63b7a` (Task 2 GREEN commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both fixes necessary for correctness. Import cycle fix is the right architectural decision — middleware and proxy types should be in a shared types package (planned for a future plan). No scope creep.

## Issues Encountered

- Go's circular import check is enforced at compile time (not link time) — the cycle was caught immediately. The fix (remove middleware fields from Handler) is consistent with the plan's note that middleware fields are "all optional (nil = skip)" and "wired in Plans 02-03 through 02-05."

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `internal/proxy/` package compiles cleanly (`go build ./...`)
- All 22 proxy tests pass (`go test ./internal/proxy/` in 5s including timeout test)
- `go vet ./...` passes with no issues
- `MCPRequestKey` context key ready for Plans 02-03 (RBAC) and 02-04 (session isolation) to read parsed method/tool from context
- `Handler.ServeHTTP` middleware hook points: currently passthrough; Plans 02-03 through 02-05 will add auth, RBAC, rate limiting, and audit logging between parse and forward
- Import cycle must be resolved before Plans 02-03 through 02-05 wire middleware into Handler — consider moving shared types to `internal/types/` package

---
*Phase: 02-mvp-build*
*Completed: 2026-03-20*
