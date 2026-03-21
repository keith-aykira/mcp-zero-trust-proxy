---
phase: 03-hardening-production-readiness
plan: 02
subsystem: auth
tags: [golang, oauth2, pkce, sse, tls, docker, cache, cleanup]

requires:
  - phase: 02-mvp-build
    provides: "OAuth Authenticator, SSE proxy, Handler, config types — all modified here"

provides:
  - "OAuth cache TTL cleanup goroutines (StartCleanup/StopCleanup)"
  - "client_secret included in PKCE token exchange when configured"
  - "Sanitized OAuth error messages — no raw provider response bodies leak"
  - "ProxySSE with configurable timeout (http.Client.Timeout) and buffer size (backpressure)"
  - "TLS termination via cert_file/key_file in config"
  - "Dockerfile updated to golang:1.25-alpine matching go.mod"

affects: [03-hardening-production-readiness, 04-beta-launch]

tech-stack:
  added: []
  patterns:
    - "Cache cleanup via dedicated runCleanup() method + StartCleanup/StopCleanup lifecycle pair"
    - "Sanitized errors: internal error details logged/discarded, generic message returned to caller"
    - "TDD: RED (compile failure) -> GREEN (pass) across both tasks"
    - "bufio.Scanner.Buffer() for configurable SSE backpressure"

key-files:
  created:
    - internal/proxy/sse_test.go
  modified:
    - internal/auth/oauth.go
    - internal/auth/oauth_test.go
    - internal/proxy/sse.go
    - internal/proxy/handler.go
    - internal/config/types.go
    - cmd/mcpproxy/main.go
    - Dockerfile

key-decisions:
  - "runCleanup() is a public-facing helper so tests can call it directly without waiting 60s for the ticker"
  - "StopCleanup uses select-on-closed pattern to avoid double-close panic (idempotent)"
  - "ProxySSE timeout=0 means no timeout — consistent with original behavior, avoids breaking existing SSE tests"
  - "maxBufferSize<=0 defaults to 64KB in both ProxySSE and NewHandler — zero-config deployments unchanged"
  - "TLS check: both cert_file AND key_file must be non-empty — partial config falls back to plain HTTP"
  - "Error sanitization in Authenticate returns 'authentication failed' — not 'token validation failed: ...' to avoid leaking fetchUserIdentity internals"
  - "exchangeCode wraps http.NewRequest error as 'token exchange failed' (sanitized) not raw error"

requirements-completed: [HARD-04, HARD-05, HARD-08, HARD-09, HARD-10, HARD-13]

duration: 14min
completed: 2026-03-21
---

# Phase 3 Plan 02: OAuth Hardening, SSE Backpressure, TLS, Docker Summary

**OAuth cache TTL cleanup, client_secret in PKCE exchange, sanitized errors, configurable SSE timeout/backpressure, TLS termination, and Dockerfile Go version fix**

## Performance

- **Duration:** 14 min
- **Started:** 2026-03-21T00:21:44Z
- **Completed:** 2026-03-21T00:35:52Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- OAuth caches have TTL-based cleanup goroutines — stateCache entries >10 min and expired tokenCache entries are removed every 60 seconds
- Client secret is included in PKCE token exchange when configured — required for providers that mandate it
- All OAuth error messages sanitized — raw provider response bodies never reach the client
- SSE proxy accepts configurable timeout and buffer size — timeout drives http.Client.Timeout; buffer size controls scanner.Buffer for backpressure on oversized lines
- TLS termination: ListenAndServeTLS used when both cert_file and key_file are configured
- Dockerfile corrected from golang:1.22-alpine to golang:1.25-alpine (matching go.mod)

## Task Commits

Each task was committed atomically:

1. **Task 1: OAuth cache cleanup, client secret support, error sanitization** - `c032d63` (feat)
2. **Task 2: SSE timeout + backpressure, TLS termination, Docker Go version** - `2d5d1c1` (feat)

**Plan metadata:** (docs commit — see final_commit step)

_Note: Both tasks used TDD — RED (compile failures confirmed), GREEN (all tests pass)_

## Files Created/Modified

- `internal/auth/oauth.go` - Added StartCleanup/StopCleanup/runCleanup methods; client_secret in exchangeCode; sanitized error messages throughout
- `internal/auth/oauth_test.go` - 8 new tests: cache cleanup (token + state), client secret present/absent, error sanitization (exchange + authenticate + callback)
- `internal/proxy/sse.go` - ProxySSE signature extended with `timeout time.Duration, maxBufferSize int`; scanner.Buffer() for configurable backpressure
- `internal/proxy/sse_test.go` - New file: 4 tests for timeout, buffer size, zero-timeout passthrough, backpressure on small buffer
- `internal/proxy/handler.go` - Handler struct gets sseTimeout/sseMaxBuffer fields; NewHandler reads from config.Server.SSE; ProxySSE call updated
- `internal/config/types.go` - TLSConfig (cert_file, key_file) and SSEConfig (timeout_seconds, max_buffer_bytes) added to ServerConfig
- `cmd/mcpproxy/main.go` - StartCleanup/StopCleanup wired; TLS branch: ListenAndServeTLS when TLS configured
- `Dockerfile` - golang:1.22-alpine -> golang:1.25-alpine

## Decisions Made

- `runCleanup()` is exported as a helper so tests can invoke it directly without waiting for a 60-second ticker tick
- `StopCleanup()` uses a select on a closed channel to be idempotent — safe to call multiple times without panic
- SSE timeout=0 preserves original behavior (no http.Client timeout); context cancellation still handles client disconnect
- maxBufferSize defaults to 64KB in both ProxySSE and NewHandler when zero/unset — zero-config behavior unchanged
- TLS requires both cert_file AND key_file — partial config silently falls back to plain HTTP to avoid startup failure
- Error sanitization in `Authenticate()` returns generic "authentication failed" — does not propagate `fetchUserIdentity` internals to the caller

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- `cmd/mcpproxy` directory is in `.gitignore` (as `mcpproxy` pattern). Used `git add -f` to force-add main.go. This is a pre-existing gitignore configuration issue (pattern `mcpproxy` matches both the binary output and the cmd subdirectory). The Go binary output `mcpproxy` at root is correctly gitignored; the source directory should not be.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- HARD-04, HARD-05, HARD-08, HARD-09, HARD-10, HARD-13 all satisfied
- Phase 3 Plan 01 (rate limiting, session cleanup, middleware, config validation) and Plan 02 (auth hardening, SSE, TLS, Docker) are complete
- Plan 03 can proceed with remaining hardening requirements

## Self-Check: PASSED

All key files present. Both task commits verified (c032d63, 2d5d1c1).

---
*Phase: 03-hardening-production-readiness*
*Completed: 2026-03-21*
