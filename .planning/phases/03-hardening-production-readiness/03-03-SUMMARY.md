---
phase: 03-hardening-production-readiness
plan: 03
subsystem: proxy, audit
tags: [golang, batch-rbac, json-rpc, audit-rotation, regression]

# Dependency graph
requires:
  - phase: 03-hardening-production-readiness
    plan: 01
    provides: "Config types (AuditRotationConfig), pipeline functional options, CORS, body size"
  - phase: 03-hardening-production-readiness
    plan: 02
    provides: "OAuth cache cleanup, SSE timeout/backpressure, TLS, error sanitization"

provides:
  - "Per-item RBAC enforcement for batch JSON-RPC requests"
  - "Audit log rotation by file size (MaxSizeMB) and/or time (MaxAgeHours)"
  - "Full regression suite — all 177 tests pass with -race flag"

affects: [04-beta-launch]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Batch detection via raw body first-byte check (isBatchRequest)"
    - "processBatch iterates items, runs RBAC per-item, forwards allowed items individually"
    - "Rotation triggered post-write inside mutex — shouldRotate/rotate pattern"
    - "bytesWritten counter (not syscall stat) for efficient size tracking"
    - "Single-backup rotation: rename to .1, open new file at original path"

key-files:
  created: []
  modified:
    - "internal/proxy/pipeline.go"
    - "internal/proxy/pipeline_test.go"
    - "internal/audit/logger.go"
    - "internal/audit/logger_test.go"
    - "tests/integration/proxy_test.go"

key-decisions:
  - "Detect batch by raw body first byte '[' (not len(reqs) > 1) — handles empty batch correctly"
  - "Forward allowed batch items individually to upstream (not as a reconstructed batch) — simpler, correct, avoids partial-batch protocol complexity"
  - "Rotation check happens post-write inside the mutex — avoids race between rotation and concurrent writes"
  - "bytesWritten counter tracks file bytes — avoids stat syscall on every write"
  - "Single .1 backup rotation — simple, sufficient for production log management"
  - "NewLoggerWithRotation constructor for test injection — avoids coupling rotation tests to config YAML"
  - "maxAgeHours < 0 in NewLoggerWithRotation means 'already expired' — sets fileCreatedAt 24h in past"

requirements-completed: [HARD-03, HARD-12, HARD-14]

# Metrics
duration: 19min
completed: 2026-03-21
---

# Phase 3 Plan 03: Batch RBAC + Audit Rotation + Full Regression Summary

**Per-item RBAC enforcement for batch JSON-RPC requests, audit log rotation by file size and time, and full regression confirming all 14 HARD requirements are met with zero regressions (177 tests, -race flag)**

## Performance

- **Duration:** 19 min
- **Started:** 2026-03-21T00:30:00Z
- **Completed:** 2026-03-21T00:49:41Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- HARD-03: Batch JSON-RPC per-item RBAC — `processBatch` in pipeline.go iterates each item independently; denied items get JSON-RPC error responses (code -32002), allowed items forwarded to upstream; each item gets its own audit entry
- HARD-12: Audit log rotation — `shouldRotate`/`rotate` methods in logger.go; rotation triggers by file size (MaxSizeMB) and/or age (MaxAgeHours); safe under concurrent writes via mutex; single .1 backup
- HARD-14: Full regression — 177 tests pass with `-race` flag (166 baseline + 11 new); zero regressions from any prior plan's changes

## Task Commits

Each task was committed atomically:

1. **Task 1: Batch JSON-RPC per-item RBAC enforcement** - `a115398` (feat) — pipeline.go processBatch + 6 new TDD tests
2. **Task 2: Audit log rotation + integration test + regression** - `f94ab72` (feat) — logger.go rotation + 4 rotation tests + 1 integration test

## Files Created/Modified

- `internal/proxy/pipeline.go` - Added `isBatchRequest`, `processBatch`; updated parse block to detect/route batch before single-request path
- `internal/proxy/pipeline_test.go` - 6 new batch RBAC unit tests: mixed allow/deny, all allowed, all denied, single unchanged, audit per item, empty batch
- `internal/audit/logger.go` - Added rotation fields (maxSizeBytes, maxAgeHours, fileCreatedAt, bytesWritten) to Logger; added shouldRotate, rotate, updateFileWriter methods; updated Log to track bytes and trigger rotation; added NewLoggerWithRotation constructor; updated NewLogger to wire cfg.Rotation
- `internal/audit/logger_test.go` - 4 new rotation tests: by size, by age, disabled, concurrent safe
- `tests/integration/proxy_test.go` - 1 new TestProxy_BatchRBAC_MixedAllowDeny test — full pipeline batch with 3 items (tools/list allowed, execute_command denied for restricted, read_file allowed); verifies response array, per-item error codes, and 3 audit entries

## Decisions Made

- Detect batch by raw body first byte `[` rather than `len(reqs) > 1` — empty batch `[]` parses to 0 items so the len check would miss it; first-byte detection is correct for all batch sizes
- Forward allowed batch items to upstream individually (not reconstructing a partial batch) — simpler implementation, avoids JSON-RPC batch spec edge cases around partial responses
- Rotation triggered post-write inside the lock — prevents a write racing with a rotation rename
- `bytesWritten` counter maintained in-memory — avoids a `stat` syscall on every write, which is hot path
- Single `.1` backup: simple single-rotation scheme; production deployments typically use a log shipper (Fluentd, Vector) that handles the `.1` file

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Empty batch handled separately (first-byte detection, not len check)**
- **Found during:** Task 1 (implementing batch RBAC — empty batch test failed)
- **Issue:** Initial implementation detected batch via `len(reqs) > 0` but empty batch `[]` returns 0 items from ParseRequest, so the batch branch was never taken — empty batch fell through to single-request path and produced no response
- **Fix:** Moved batch detection to `isBatchRequest(bodyBytes)` (first-byte `[` check) BEFORE parsing, then route to `processBatch` with whatever reqs were parsed (including empty slice)
- **Files modified:** internal/proxy/pipeline.go
- **Commit:** a115398

**2. [Rule 1 - Bug] NewLoggerWithRotation age expiry was computing future timestamp**
- **Found during:** Task 2 (TestRotation_ByAge failed)
- **Issue:** Formula `time.Duration(-maxAgeHours*2) * time.Hour` with `maxAgeHours=-1` produced `+2 hours` (future), so the "already expired" signal was actually setting fileCreatedAt in the future, meaning shouldRotate never triggered
- **Fix:** Changed to explicit: `l.maxAgeHours = 1` and `l.fileCreatedAt = time.Now().Add(-24 * time.Hour)` — clear intent, always past the 1-hour limit
- **Files modified:** internal/audit/logger.go
- **Commit:** f94ab72

---

**Total deviations:** 2 auto-fixed (Rule 1 — bugs found and fixed during TDD GREEN phase)
**Impact on plan:** Both fixes were necessary for correctness; plan intent fully met.

## Hardening Requirements Summary

| Requirement | Description | Satisfied By Plan |
|-------------|-------------|-------------------|
| HARD-01 | User-to-role mapping from YAML | Plan 01 |
| HARD-02 | Body size enforcement (413) | Plan 01 |
| HARD-03 | Batch JSON-RPC per-item RBAC | **Plan 03** |
| HARD-04 | OAuth cache TTL cleanup | Plan 02 |
| HARD-05 | Client secret in PKCE exchange | Plan 02 |
| HARD-06 | CORS middleware with OPTIONS preflight | Plan 01 |
| HARD-07 | X-Request-ID on all responses | Plan 01 |
| HARD-08 | Sanitized OAuth error messages | Plan 02 |
| HARD-09 | Configurable SSE timeout | Plan 02 |
| HARD-10 | SSE backpressure (buffer size) | Plan 02 |
| HARD-11 | Single body parse (Handler pass-through) | Plan 01 |
| HARD-12 | Audit log rotation by size/time | **Plan 03** |
| HARD-13 | TLS termination (cert_file/key_file) | Plan 02 |
| HARD-14 | Full regression, zero regressions | **Plan 03** |

All 14 HARD requirements satisfied. 177 tests passing with -race flag.

## Issues Encountered

None beyond the two auto-fixed bugs noted above.

## Next Phase Readiness

- All 14 HARD requirements from the Phase 3 audit are satisfied
- Phase 3 is complete — all 3 plans done
- Phase 4 (Beta Launch & First Revenue) can proceed
- Production proxy is hardened: RBAC (including batch), audit rotation, CORS, rate limiting, TLS, OAuth, body size limits, single-parse, error sanitization

---
*Phase: 03-hardening-production-readiness*
*Completed: 2026-03-21*
