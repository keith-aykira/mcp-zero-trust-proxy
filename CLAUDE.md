# CLAUDE.md — Project Context for AI Tools

## Project: MCP Zero-Trust Proxy

A drop-in reverse proxy that adds OAuth 2.1 PKCE authentication, tool-level RBAC, session isolation, rate limiting, and structured audit logging to any MCP server — with zero code changes to the protected server.

**Domain:** mcpzerotrust.dev (live on Vercel)
**Owner:** Andrew Noble (andrewnoble1992@gmail.com)
**Stage:** Phases 0-5 complete. Phase 6 (Marketing & SEO) in progress. Pre-launch — production Stripe keys + Show HN remain.

## Commands

```bash
# Go binary (not system-wide — MUST use this path)
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"

# Build
go build -o mcpproxy ./cmd/mcpproxy

# Test (all 236+ tests across 9 packages)
go test ./...

# Test one package
go test ./internal/proxy/ -v
go test ./tests/e2e/ -v
go test ./tests/integration/ -v

# Run with config
./mcpproxy --config ./configs/example.yaml

# Docker
docker build -t mcp-zero-trust-proxy .
docker run -p 8080:8080 -v ./config.yaml:/etc/mcpproxy/config.yaml mcp-zero-trust-proxy

# Deploy landing page
cd landing-page && vercel --prod --yes

# Deploy Edge Functions
supabase functions deploy stripe-webhook --no-verify-jwt --project-ref dwumoznjyckebuirghne
supabase functions deploy create-license --no-verify-jwt --project-ref dwumoznjyckebuirghne
supabase functions deploy get-license --no-verify-jwt --project-ref dwumoznjyckebuirghne
```

## Current State

- **GitHub**: AnobleSCM/mcp-zero-trust-proxy (private)
- **Supabase**: project ref `dwumoznjyckebuirghne` — waitlist + licenses tables, 3 Edge Functions deployed
- **Landing page**: Live at mcpzerotrust.dev (Vercel), Mercury-style design, SEO optimized
- **Stripe**: Test mode configured (products, webhook, payment links). Live keys needed for launch.
- **Tests**: 236+ tests passing across 9 packages (unit + integration + E2E persona validation)
- **Docker**: 6.6MB image, sub-millisecond latency (p50=386us)
- **GSD**: `.planning/` — 6 phases, Phases 0-4 code complete, Phase 5 (E2E validation) complete

## Immediate Priorities

1. **Phase 6: Marketing & SEO** — comparison pages, FAQ schema, community seeding, AI discoverability
2. **Production Stripe** — swap test keys for live (needs Andrew's `sk_live_` key)
3. **Show HN** — post ready at `docs/go-to-market/SHOW-HN.md`, launch weekday 8-10am ET
4. **v1.0.0 tag** — triggers GHCR Docker image + GitHub Release via Actions

## Package Map

```
cmd/mcpproxy/main.go          — Entry point, wires all components, tier enforcement
internal/
  auth/                        — OAuth 2.1 PKCE, GitHub/Google/OIDC providers, session store
  proxy/                       — HTTP handler, reverse proxy, JSON-RPC parser, SSE, pipeline, CORS
  rbac/                        — Role engine (admin/readonly/restricted + custom), tool filtering
  ratelimit/                   — Per-client token bucket rate limiter
  audit/                       — JSONL logger with file rotation by size/age
  license/                     — JWT license validation, tier enforcement (Free/Pro/Enterprise)
  config/                      — YAML loader, env var substitution, validation
  middleware/                   — Interface definitions
tests/
  integration/                 — 20+ tests with mock MCP servers (5 server types)
  e2e/                         — 24 persona validation tests (Marcus/Priya/James/Sofia/Kai)
supabase/functions/
  stripe-webhook/              — Stripe event handler → triggers license creation
  create-license/              — Generates ECDSA-signed JWT license keys
  get-license/                 — Retrieves license by Stripe session ID
landing-page/                  — Static site on Vercel (index.html, checkout-success.html, SEO files)
```

## Key Files

- `docs/QUICKSTART.md` — Docker + binary install, config, OAuth setup, RBAC, license tiers, upgrade path
- `docs/CONFIG-REFERENCE.md` — Full YAML schema with types, defaults, examples
- `docs/MULTI-TENANT.md` — Agency setup with Docker Compose (one proxy per client)
- `docs/e2e-validation/persona-validation-report.md` — Ship/no-ship assessment from 5 buyer personas
- `docs/go-to-market/SHOW-HN.md` — Hacker News launch post draft
- `docs/research/` — Pain signals, competitive analysis, ecosystem pulse, buyer behavior, go/no-go brief
- `configs/example.yaml` — Full annotated config example
- `HANDOFF_CURRENT.md` — Session handoff with current status and next actions
- `.planning/` — GSD tracking (PROJECT.md, REQUIREMENTS.md, ROADMAP.md, STATE.md)

## Research Findings (Phase 1 — 2026-03-19)

- **Pain:** 15 genuine developer complaints (active breaches, 220K+ exposed instances, 30 CVEs in 60 days)
- **Competition:** No turnkey competitor (simple + enterprise + transparent pricing). IBM ContextForge closest but needs K8s.
- **Ecosystem:** 52M+/mo PyPI downloads, ~4K+ servers. Auth added to spec but optional.
- **Buyers:** $49+/mo viable. Gap between free (sigbit) and enterprise (Kong $500+, Lunar $250/gateway).
- **Decision: GO** — all 5 kill criteria passed. Window is 3-6 months.

## Tech Stack

- **Proxy:** Go 1.26 (single binary, ~10MB)
- **Billing:** Stripe (test mode) + Supabase Edge Functions (Deno)
- **Database:** Supabase Postgres (licenses, waitlist)
- **Auth:** OAuth 2.1 PKCE (built-in, stdlib only — no external JWT library)
- **Hosting:** Vercel (landing page) + Docker/GHCR (proxy) + Supabase (Edge Functions)
- **CI/CD:** GitHub Actions (release workflow on tag push)

## Architecture Notes

The proxy sits between MCP clients (Claude, Cursor, Copilot) and MCP servers. Pipeline order:

1. Body size check → 2. Auth (Bearer token → OAuth provider) → 3. Rate limit (per-client token bucket) → 4. JSON-RPC parse → 5. RBAC check (role × method × tool) → 6. Forward to upstream → 7. Filter tools/list response → 8. Audit log

MCP uses JSON-RPC 2.0 over HTTP+SSE. Key methods: `initialize`, `tools/list`, `tools/call`, `resources/read`, `prompts/get`.

## Gotchas

- **Go binary path**: Not system-wide. Must use `/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin/go`
- **Free tier overrides config**: When no license key is set, `main.go` forces RPM=10, burst=5, audit=stdout-only regardless of YAML
- **User-role mapping uses email keys**: `userRoles[email]` not username — config must use `"alice@co.com": "admin"` format
- **Content-Length on filtered tools/list**: Pipeline recalculates after RBAC filtering — was a bug, fixed in E2E validation
- **License signing key**: `keys/private_key.pem` is gitignored. ECDSA P-256. Must be in Supabase secrets as `LICENSE_SIGNING_KEY`
- **Stripe test vs live**: Currently test mode. Payment links in index.html are `buy.stripe.com/test_*` — must swap before launch

## Conventions

- Conventional commits (feat:, fix:, docs:, chore:, test:)
- Go standard project layout
- Config via YAML, secrets via env vars only
- Docs in Markdown in `/docs/`
- Notion project page: `329532af-14ea-81fd-94a0-e88881c9d68e`

## What NOT To Do

- Don't build dashboard UI yet (CLI/config only for MVP)
- Don't implement SSO/SAML yet (Enterprise tier, future)
- Don't proxy stdio transport (HTTP+SSE only)
- Don't hardcode credentials in source files
- Don't make the GitHub repo public yet (decision: stays private for launch)
