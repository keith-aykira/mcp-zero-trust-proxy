## Current Position

Phase: 2 — MVP Build
Plan: 02 of 06 COMPLETE
Status: IN PROGRESS — Plan 02-02 complete, MCP reverse proxy core done
Last activity: 2026-03-20 — 02-02 executed: JSON-RPC 2.0 parser, HTTP+SSE reverse proxy, 22 TDD tests pass

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-19)

**Core value:** Any MCP server can be protected with enterprise-grade security in 5 minutes via Docker — no code changes required.
**Current focus:** Phase 2 — MVP Build (Plans 01-02 done, Plan 03 up next: Tool-level RBAC enforcement)

## Phase 0 Progress (COMPLETE)

- [x] INFRA-02: Git repo initialized and pushed to GitHub (private)
- [x] INFRA-03: Supabase project with waitlist table and RLS policies
- [x] INFRA-01: Landing page wired to Supabase (deploy deferred — not public yet)

## Phase 1 Progress (COMPLETE)

- [x] Pain signal analysis — 14 genuine complaints found, pain is real and escalating → `docs/research/PAIN-SIGNALS.md`
- [x] Competitor movement report — 12 competitors updated + 4 new entrants, no turnkey solution in our lane → `docs/research/COMPETITIVE-ANALYSIS.md`
- [x] MCP ecosystem pulse — 52M+/mo PyPI downloads, ~4K servers, enterprise adoption accelerating → `docs/research/MCP-ECOSYSTEM-PULSE.md`
- [x] Buyer behavior analysis — $49+/mo viable, pricing benchmarks mapped across 4 segments → `docs/research/BUYER-BEHAVIOR.md`
- [x] Technical landscape update — 6 new CVEs, new attack vectors, no mandatory spec auth → `docs/research/TECHNICAL-LANDSCAPE.md`
- [x] Go/no-go brief — ALL 5 KILL CRITERIA PASS → DECISION: GO → `docs/research/GO-NO-GO-BRIEF.md`

## Accumulated Context

- Supabase project ref: dwumoznjyckebuirghne
- Supabase URL: https://dwumoznjyckebuirghne.supabase.co
- GitHub repo: AnobleSCM/mcp-zero-trust-proxy (private)
- Domain: mcpzerotrust.dev (purchased via Vercel, not yet connected)
- Waitlist form: wired to Supabase, verified working (test insert successful)
- RLS: anon can insert, only authenticated can read
- Landing page deployment deferred per user decision (not public yet)
- Notion project page: 329532af-14ea-81fd-94a0-e88881c9d68e
- Research approach: Deep research over outreach — Exa deep research pro, advanced search, company research
- Phase 1 research cost: ~$6.86 across 5 Exa deep_researcher_pro queries
- Go/no-go decision: GO — all 5 kill criteria passed
- Research files: docs/research/ (PAIN-SIGNALS, COMPETITIVE-ANALYSIS, MCP-ECOSYSTEM-PULSE, BUYER-BEHAVIOR, TECHNICAL-LANDSCAPE, GO-NO-GO-BRIEF)
- Key risk: Window is 3-6 months before market consolidates (IBM adding features, PointGuard AI entered Mar 18)
- Go binary path: /Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin/go (go 1.26.0, no system-wide install)
- Go module: github.com/AnobleSCM/mcp-zero-trust-proxy
- Core interfaces stable in internal/proxy/ and internal/middleware/ — all plans 02-06 build against these
- Three built-in RBAC roles: admin (all tools), readonly (list+read), restricted (tools/list only)
- Config loads from YAML with ${ENV_VAR} substitution; defaults: listen :8080, 100 req/min, burst 10, audit stdout enabled

## Phase 2 Progress (IN PROGRESS)

- [x] 02-01: Go module bootstrap — interfaces, config system, 10 TDD tests, example YAML
- [x] 02-02: MCP reverse proxy core — JSON-RPC 2.0 parser, HTTP+SSE proxy, 22 TDD tests
- [ ] 02-03: Tool-level RBAC enforcement
- [ ] 02-04: Session isolation
- [ ] 02-05: Rate limiting + audit logging
- [ ] 02-06: HTTP reverse proxy + SSE transport wiring

## Decisions Log

- 2026-03-20 (02-01): Three built-in roles (admin/readonly/restricted) are the only valid role names — strict validation
- 2026-03-20 (02-01): Audit defaults to enabled=true — zero-config deployments get full audit logging
- 2026-03-20 (02-01): Load() applies defaults, Validate() checks required fields — two separate passes
- 2026-03-20 (02-01): Go binary at cache path (not system-wide) — must set in PATH for all Go commands
- 2026-03-20 (02-02): Import cycle between proxy and middleware packages — handler.go cannot import middleware; middleware fields deferred to Plans 02-03 through 02-05
- 2026-03-20 (02-02): Context values don't cross HTTP connections — MCPRequestKey is for in-process middleware only; test verification uses SetTransport hook
- 2026-03-20 (02-02): httputil.ReverseProxy for HTTP, custom ProxySSE for SSE — ReverseProxy buffers which breaks SSE streaming
- 2026-03-20 (02-02): Body buffering pattern: io.ReadAll + io.NopCloser(bytes.NewReader) — makes body readable for both parse and proxy forward
- 2026-03-20 (02-02): SetTransport exported on Handler — enables test transport injection without exposing reverseProxy field directly
