---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: executing
last_updated: "2026-03-23T22:08:20Z"
last_activity: 2026-03-23 — Plan 06-03 complete (landing page copy hardening; PLH-03, PLH-04, PLH-06, PLH-07, PLH-08, PLH-09 satisfied)
progress:
  total_phases: 7
  completed_phases: 6
  total_plans: 22
  completed_plans: 22
---

## Current Position

Phase: 6 — Pre-Launch Hardening
Plan: 06-03 (COMPLETE — landing page copy hardening, citations, FAQ, open-source decision)
Status: IN PROGRESS — Phase 6 plan 03 done. Test count corrected (223), threat stats cited, native auth FAQ added, superlatives removed, OPEN-SOURCE-DECISION.md created.
Last activity: 2026-03-23 — Plan 06-03 complete (landing page copy hardening; PLH-03, PLH-04, PLH-06, PLH-07, PLH-08, PLH-09 satisfied)

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-19)

**Core value:** Any MCP server can be protected with enterprise-grade security in 5 minutes via Docker — no code changes required.
**Current focus:** Phase 4 — Beta Launch & First Revenue (Stripe billing + landing page deploy + outreach)

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

### Roadmap Evolution

- Phase 3 added: Hardening & Production Readiness (14 production gaps from Phase 2 audit)
- Beta Launch & First Revenue renumbered from Phase 3 → Phase 4
- Phase 5 added: Marketing, SEO & Discoverability — search, social, AI search engines, community

## Phase 2 Progress (COMPLETE)

- [x] 02-01: Go module bootstrap — interfaces, config system, 10 TDD tests, example YAML
- [x] 02-02: MCP reverse proxy core — JSON-RPC 2.0 parser, HTTP+SSE proxy, 22 TDD tests
- [x] 02-03: OAuth 2.1 PKCE auth — Authenticator, PKCE utils, GitHub/Google/OIDC providers, SessionStore, 31 TDD tests
- [x] 02-04: RBAC engine + audit logger + rate limiter — 3 roles, JSONL audit, token bucket, 36 TDD tests
- [x] 02-05: Proxy pipeline wiring — middleware chain, Docker image, CI, 9 TDD tests
- [x] 02-06: Integration tests (18 tests, 5 MCP server types) + QUICKSTART.md + CONFIG-REFERENCE.md

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
- 2026-03-20 (02-03): Standard library net/http for OAuth calls — no golang.org/x/oauth2 dependency; simpler PKCE code exchange
- 2026-03-20 (02-03): Default provider falls back to GitHub when AuthConfig.Provider is empty — avoids nil panic in tests
- 2026-03-20 (02-03): Token cache uses sync.Map (lock-free reads) with tokenCacheEntry{identity, expiresAt} — auth is hot path
- 2026-03-20 (02-03): Session TTL refreshed on each Get() access — active sessions don't expire mid-use
- 2026-03-20 (02-03): Google TokenURL is oauth2.googleapis.com/token (not accounts.google.com) — test corrected to match actual endpoint
- 2026-03-20 (02-04): initialize method always allowed for all roles — required for MCP handshake
- 2026-03-20 (02-04): ReadOnly uses denyToolsCall flag (not per-tool allow list) — cleaner than listing all tools to deny
- 2026-03-20 (02-04): Audit logger uses injectable io.Writer for testability without file system dependency
- 2026-03-20 (02-04): Rate limiter uses atomic.Int64 for lastAccess (not time.Time) — prevents race on sync.Map fast-path
- 2026-03-20 (02-04): golang.org/x/time v0.9.0 added — Plan 01 SUMMARY.md incorrectly listed it as already present
- 2026-03-20 (02-05): AuditEntry/AuditLogger moved to proxy package — middleware uses type aliases (=) for backward compat; resolves import cycle
- 2026-03-20 (02-05): proxy_test external package for pipeline tests — avoids middleware->proxy import cycle in test files
- 2026-03-20 (02-05): responseCapture pattern for tools/list filtering — custom ResponseWriter buffers upstream response for RBAC post-processing
- 2026-03-20 (02-05): Binary size 9.8MB with CGO_ENABLED=0, -s -w flags — well under 20MB limit
- 2026-03-20 (02-06): testAuthenticator (mock, not real OAuth) for integration tests — real OAuth requires live endpoints; mock tests the pipeline's auth integration path
- 2026-03-20 (02-06): mutex-protected testAuditLogger — concurrent HTTP handlers require sync.Mutex on any shared state; slice append without lock is a data race
- 2026-03-20 (02-06): SSEStreamingServer checks request body method, not URL path — ReverseProxy Director overwrites upstream path with r.URL.Path from incoming request
- 2026-03-21 (03-01): Functional options (WithMaxBodySize, WithCORS) for Pipeline — avoids breaking all callers with new required params
- 2026-03-21 (03-01): CORSConfig duplicated in proxy package (not imported from config) — avoids adding config dep to proxy package, prevents import cycle
- 2026-03-21 (03-01): OPTIONS preflight handled in ServeHTTP before auth — prevents CORS preflight from triggering 401 Unauthorized
- 2026-03-21 (03-01): LimitReader wraps body for chunked transfers (ContentLength=-1) — handles overflow detection without known size
- 2026-03-21 (03-01): Handler body parsing removed entirely (HARD-11) — Pipeline is the single owner of body reading and MCPRequestKey injection
- 2026-03-21 (03-01): ErrCodeRequestTooLarge (-32004) added as new JSON-RPC error code for 413 responses
- 2026-03-21 (03-03): Detect batch by raw body first byte '[' not len(reqs) — handles empty batch correctly
- 2026-03-21 (03-03): Forward allowed batch items individually to upstream — avoids partial-batch protocol complexity
- 2026-03-21 (03-03): Audit rotation triggered post-write inside mutex — prevents race between write and rename
- 2026-03-21 (03-03): bytesWritten counter for rotation size tracking — avoids stat syscall on every write
- 2026-03-21 (03-03): Single .1 backup rotation scheme — simple, sufficient for production log management
- 2026-03-21 (04-01): stdlib-only JWT — crypto/ecdsa + encoding/asn1 + crypto/x509 (no external JWT library); reduces attack surface
- 2026-03-21 (04-01): ASN.1 DER signature encoding — matches openssl output format for offline key generation
- 2026-03-21 (04-01): ParseEmbedded() as main.go API — binary self-contained via go:embed; no runtime key file required
- 2026-03-21 (04-01): Tier enforcement via config field overrides at startup — simpler than middleware checks; avoids adding license checks throughout chain
- 2026-03-21 (04-01): Private key never committed — keys/license-signing-private.pem excluded from git
- 2026-03-21 (04-02): Deno crypto.subtle only for JWT signing (ECDSA ES256) and Stripe webhook sig verification (HMAC-SHA256) — no SDK, lightweight Edge Functions
- 2026-03-21 (04-02): 0 = unlimited for enterprise tier max_upstreams/max_rpm — matches Plan 01 proxy validation convention
- 2026-03-21 (04-02): tier must be in Stripe metadata (checkout session or payment link) — no server-side tier lookup required at webhook time
- 2026-03-21 (04-02): CREATE_LICENSE_FUNCTION_URL secret for internal function-to-function calls — stripe-webhook invokes create-license via HTTP fetch
- 2026-03-21 (04-02): PKCS#8 PEM format for signing private key — required by crypto.subtle.importKey('pkcs8')
- 2026-03-21 (04-02): Service role RLS only for license inserts/updates — anon read is intentional (JWT is the secret, not the row)
- 2026-03-21 (04-03): Stripe checkout links use named placeholders (STRIPE_PRO_LINK / STRIPE_ENTERPRISE_LINK) — agent generates page without live credentials; user replaces before deploy
- 2026-03-21 (04-03): GHCR auth uses GITHUB_TOKEN only — no PAT required, works out of the box for any push to AnobleSCM org
- 2026-03-21 (04-03): Show HN body leads with breach data (8,000+ exposed servers, 30+ CVEs) — evidence-first hook for technical audience
- 2026-03-21 (04-03): "Founding member pricing — locked in for life" badge above pricing grid — urgency framing for early adopters
- 2026-03-23 (05-01): FAQPage questions cover 5 buyer intents: setup, comparison, self-hosting, pricing, client compatibility — these match exact developer search queries
- 2026-03-23 (05-01): Comparison page sitemap entries added before pages exist — search engines pre-discover URLs, crawl happens immediately on page publish
- 2026-03-23 (05-01): SoftwareApplication enhanced with downloadUrl pointing to GitHub Releases — supports direct download indexing and AI citation eligibility
- 2026-03-23 (05-02): Honest competitor framing on comparison pages — each page may direct users to competitor if better fit; builds trust with skeptical developer audience
- 2026-03-23 (05-02): Inlined CSS only on comparison pages — fully self-contained, no external stylesheet dependency beyond Google Fonts CDN
- 2026-03-23 (05-03): UTM link placeholders [USE UTM LINK FOR THIS CHANNEL] added to outreach templates — prevents using wrong tracked link per channel
- 2026-03-23 (05-03): awesome-mcp-servers PR is highest priority registry submission — most-starred MCP resource list, crawled by AI search engines
- 2026-03-23 (05-03): Registry submission order: awesome-mcp-servers -> Smithery -> Official MCP Registry (by traffic volume, then review speed)
- 2026-03-23 (06-03): Closed-source for v1.0 — competitive copying risk outweighs trust benefit before brand recognition established; review at 90-day milestone
- 2026-03-23 (06-03): Native auth objection addressed in both JSON-LD schema and visible FAQ section — universal HN question for security tools, not answering it would be a red flag
- 2026-03-23 (06-03): Factual claim language: "Tested with Go's race detector" replaces "Race-condition free" — more credible to technical audiences

- 2026-03-23 (06-01): Free tier rate limit increased from 10 to 60 req/min — AI agent workflows (Claude/Cursor) exhaust 10 req/min in seconds, making product appear broken to first-time users
- 2026-03-23 (06-01): Free tier burst increased from 5 to 30 — multi-tool MCP sessions fire several requests in quick succession; 5-burst was causing false rate limit errors
