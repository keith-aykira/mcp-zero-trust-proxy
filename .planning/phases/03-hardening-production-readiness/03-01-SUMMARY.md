---
phase: 03-hardening-production-readiness
plan: 01
subsystem: proxy
tags: [go, cors, body-limit, request-id, rbac, oauth, config]

# Dependency graph
requires:
  - phase: 02-mvp-build
    provides: "Pipeline, Handler, Authenticator, RBAC engine, config system with YAML loading"
provides:
  - "Configurable user-to-role mapping via YAML user_roles block"
  - "Body size enforcement (413 rejection) before any parsing"
  - "CORS middleware with OPTIONS preflight support"
  - "X-Request-ID header on all responses (success and error)"
  - "Single body parse — Handler no longer parses; Pipeline owns it exclusively"
  - "Config types: UserRolesConfig, CORSConfig, AuditRotationConfig, MaxBodySize"
affects: [03-02, 03-03, 04-beta-launch]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Functional options (PipelineOption) pattern for pipeline configuration"
    - "CORSConfig exported from proxy package for test and wiring access"
    - "LimitReader + ContentLength dual-check for body size enforcement"
    - "Options preflight handled before auth to avoid 401 on CORS checks"

key-files:
  created: []
  modified:
    - "internal/config/types.go"
    - "internal/config/config.go"
    - "internal/config/config_test.go"
    - "internal/auth/oauth.go"
    - "internal/auth/oauth_test.go"
    - "internal/proxy/pipeline.go"
    - "internal/proxy/pipeline_test.go"
    - "internal/proxy/handler.go"
    - "internal/proxy/handler_test.go"
    - "internal/proxy/jsonrpc.go"
    - "cmd/mcpproxy/main.go"

key-decisions:
  - "Functional options (WithMaxBodySize, WithCORS) for Pipeline — avoids breaking all callers with new required params"
  - "CORSConfig duplicated in proxy package (not imported from config) — avoids adding config dep to proxy package"
  - "OPTIONS preflight handled in ServeHTTP before auth — prevents CORS preflight from hitting 401"
  - "LimitReader wraps body for chunked transfers (ContentLength=-1) — handles overflow detection without ContentLength"
  - "Handler body parsing removed entirely (HARD-11) — Pipeline is the single owner of body reading and MCPRequestKey injection"
  - "ErrCodeRequestTooLarge (-32004) added as new JSON-RPC error code for 413 responses"

patterns-established:
  - "Pipeline functional options: use WithXxx functions to configure optional pipeline features"
  - "CORS before auth: handle preflight in ServeHTTP before routing to runPipeline"
  - "X-Request-ID set as first header write in runPipeline — guarantees presence on all response paths"

requirements-completed: [HARD-01, HARD-02, HARD-06, HARD-07, HARD-11]

# Metrics
duration: 18min
completed: 2026-03-21
---

# Phase 3 Plan 01: Hardening — Config Types + Pipeline Hardening Summary

**Configurable YAML user-role mapping, 413 body size enforcement, CORS middleware with OPTIONS preflight, X-Request-ID on all responses, and single body parse (Handler no longer parses)**

## Performance

- **Duration:** 18 min
- **Started:** 2026-03-21T00:21:02Z
- **Completed:** 2026-03-21T00:39:00Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments

- HARD-01: User-to-role mapping loaded from YAML `user_roles` block; Authenticator.SetUserRoles wired from config in main.go
- HARD-02: Body size enforcement in runPipeline — rejects requests over MaxBodySize with 413 before auth/RBAC/parse
- HARD-06: CORS middleware with configurable origins/methods/headers/max-age; OPTIONS preflight gets 204 without hitting auth
- HARD-07: X-Request-ID header set on ALL responses before any writes in runPipeline
- HARD-11: Handler.ServeHTTP no longer reads or parses body — Pipeline owns the single parse exclusively

## Task Commits

Each task was committed atomically:

1. **Task 1: Config types + user-to-role mapping** - `f0a1047` (chore) + `af2c42f` (fix) — prior session commits contain the bulk of implementation; this session confirmed tests pass and wired main.go
2. **Task 2: Pipeline hardening** - `ecc8ee5` (feat) — CORS, X-Request-ID, body limit, single-parse

**Plan metadata:** see final docs commit (created after SUMMARY)

_Note: Task 1 implementation was partially present in prior commits (af2c42f, f0a1047). This plan execution verified the implementation, added comprehensive failing tests confirming RED, confirmed GREEN, and committed the main.go wiring._

## Files Created/Modified

- `internal/config/types.go` - Added UserRolesConfig, CORSConfig, AuditRotationConfig; MaxBodySize in ServerConfig
- `internal/config/config.go` - Added defaults for MaxBodySize (1MB), UserRoles.Default ("readonly"); validation for user_roles mapping
- `internal/config/config_test.go` - 10 new tests for UserRoles, MaxBodySize, CORS, TLS, SSE, AuditRotation
- `internal/auth/oauth.go` - SetUserRoles(), resolveRole() methods; Authenticate() uses resolveRole; RWMutex for concurrent access
- `internal/auth/oauth_test.go` - 4 new tests for resolveRole and Authenticate role resolution
- `internal/proxy/pipeline.go` - PipelineOption, WithMaxBodySize, WithCORS, CORSConfig; body size enforcement; X-Request-ID; CORS middleware; OPTIONS preflight; functional options on NewPipeline/NewPipelineForTest
- `internal/proxy/pipeline_test.go` - 8 new tests for body size, request ID, CORS behavior, handler single-parse
- `internal/proxy/handler.go` - Removed body reading and JSON-RPC parsing from ServeHTTP (now pass-through only)
- `internal/proxy/handler_test.go` - Updated TestHandler_ParsedMCPRequest_InContext to verify Handler does NOT set context key
- `internal/proxy/jsonrpc.go` - Added ErrCodeRequestTooLarge (-32004)
- `cmd/mcpproxy/main.go` - Wired SetUserRoles, WithMaxBodySize, WithCORS from config to Pipeline

## Decisions Made

- Functional options (WithMaxBodySize, WithCORS) for Pipeline — avoids changing all existing callers' positional args
- CORSConfig exported from proxy package (not imported from config) — keeps config package out of proxy's import graph
- OPTIONS preflight handled in ServeHTTP before runPipeline — prevents CORS preflight from triggering 401 Unauthorized
- LimitReader wraps body for chunked transfers where ContentLength=-1 — handles overflow detection without known size
- Handler body parsing removed entirely — Pipeline is the single owner of body reading and MCPRequestKey injection
- ErrCodeRequestTooLarge (-32004) added as MCP-specific error code for semantically correct 413 responses

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated TestHandler_ParsedMCPRequest_InContext to reflect new single-parse behavior**
- **Found during:** Task 2 (pipeline hardening — remove Handler body parse)
- **Issue:** Existing test expected Handler to set MCPRequestKey in context; this test became wrong when HARD-11 removed body parsing from Handler
- **Fix:** Renamed test to TestHandler_DoesNotParseBody_InContext and inverted the assertion — now verifies MCPRequestKey is absent
- **Files modified:** internal/proxy/handler_test.go
- **Verification:** All proxy tests pass
- **Committed in:** ecc8ee5 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug in test that contradicted the intended behavior of HARD-11)
**Impact on plan:** Essential fix — test accurately reflects the new single-parse contract.

## Issues Encountered

- Task 1 implementation was mostly present from prior session commits (`af2c42f`, `f0a1047`) — this session added the new failing tests to confirm RED, verified GREEN, and ensured main.go wiring was complete. The `03-01-SUMMARY.md` was never created, so this plan was re-run to formalize completion.

## Next Phase Readiness

- All 5 HARD requirements (01, 02, 06, 07, 11) from Plan 01 are satisfied
- Config system is expanded with all hardening fields; Plan 02 and 03 can now use these types
- Pipeline hardening foundation is in place for remaining plans in Phase 03

---
*Phase: 03-hardening-production-readiness*
*Completed: 2026-03-21*

## Self-Check: PASSED

Files verified:
- `internal/proxy/pipeline.go` — FOUND (ecc8ee5)
- `internal/proxy/handler.go` — FOUND (ecc8ee5)
- `internal/config/types.go` — FOUND (af2c42f)
- `internal/auth/oauth.go` — FOUND (af2c42f)
- Task 2 commit `ecc8ee5` — confirmed in git log
