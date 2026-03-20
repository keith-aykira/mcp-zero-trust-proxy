# CLAUDE.md — Project Context for AI Tools

## Project: MCP Zero-Trust Proxy

A drop-in reverse proxy that adds OAuth 2.1 PKCE authentication, tool-level RBAC, session isolation, rate limiting, and immutable audit logging to any MCP server — with zero code changes to the protected server.

**Domain:** mcpzerotrust.dev
**Owner:** Andrew Noble (andrewnoble1992@gmail.com)
**Stage:** Phase 1 complete (deep research done, GO decision made). Ready for Phase 2: MVP Build.

## Current State

- **GitHub**: AnobleSCM/mcp-zero-trust-proxy (private)
- **Supabase**: project ref `dwumoznjyckebuirghne` — waitlist table with RLS (anon insert, auth read)
- **Landing page**: Wired to Supabase, NOT deployed to Vercel yet (deliberate — not going public yet)
- **Domain**: mcpzerotrust.dev purchased on Vercel, not connected yet
- **GSD**: Initialized in `.planning/` — 4 phases, 24 requirements

## Immediate Priorities

1. **Deploy landing page to Vercel** — Connect mcpzerotrust.dev, start collecting waitlist signups
2. **Phase 2: MVP Build** — Go-based reverse proxy with OAuth 2.1 PKCE, RBAC (3 roles), audit logging, session isolation
3. **Phase 3: Beta Launch** — 10+ teams in production, first revenue

## Research Findings (Phase 1 — 2026-03-19)

- **Pain:** 15 genuine developer complaints (active breaches, 220K+ exposed instances, 30 CVEs in 60 days)
- **Competition:** No turnkey competitor in our lane (simple + enterprise + transparent pricing). IBM ContextForge closest but needs K8s.
- **Ecosystem:** 52M+/mo PyPI downloads, ~4K+ servers. MCP is the de facto standard. Auth added to spec but optional.
- **Buyers:** $49+/mo viable. Gap between free (sigbit) and enterprise (Kong $500+, Lunar $250/gateway) is wide.
- **Tech landscape:** 6 new CVEs in Feb-Mar 2026, new AI-native attack vectors. Threat is GROWING.
- **Decision: GO** — all 5 kill criteria passed. Window is 3-6 months.

## Key Files

- `HANDOFF.md` — Complete handoff doc with step-by-step setup instructions, Supabase SQL, Vercel deploy commands
- `PROJECT-BRIEF.md` — Product spec, architecture diagram, tech stack, MVP scope, competitive gaps
- `docs/ROADMAP.md` — 90-day roadmap with 4 phases, milestones, kill criteria, revenue projections
- `docs/research/COMPETITIVE-ANALYSIS.md` — 12 competitors analyzed with feature gap matrix
- `docs/research/ATTACK-SURFACE.md` — Threat landscape: CVEs, breaches, exposed server counts
- `docs/go-to-market/OUTREACH-MESSAGES.md` — Pre-written Discord, DM, Reddit, HN, and email templates
- `landing-page/index.html` — Complete landing page (wired to Supabase, ready to deploy)
- `.planning/` — GSD project tracking (PROJECT.md, REQUIREMENTS.md, ROADMAP.md, STATE.md)

## Tech Stack

- **Proxy:** Go (single binary)
- **Dashboard:** Next.js + Tailwind (later phase)
- **Database:** SQLite (dev) / Postgres via Supabase (prod)
- **Auth:** OAuth 2.1 PKCE (built-in)
- **Billing:** Stripe
- **Hosting:** Vercel (landing page + dashboard) + Docker (proxy)
- **CI/CD:** GitHub Actions

## Architecture Notes

The proxy sits between MCP clients (Claude, Cursor, Copilot) and MCP servers. It:
1. Terminates the client connection
2. Validates OAuth tokens (PKCE flow)
3. Checks tool-level RBAC policies against the JSON-RPC method + params
4. Enforces session isolation (per-client boundaries)
5. Rate limits per client
6. Logs every call to the audit trail
7. Forwards authorized requests to the upstream MCP server

MCP uses JSON-RPC 2.0 over HTTP+SSE (Streamable HTTP). Key methods to intercept: `initialize`, `tools/list`, `tools/call`, `resources/read`, `prompts/get`.

## Conventions

- Commits should be conventional commits (feat:, fix:, docs:, chore:)
- Go code follows standard Go project layout
- All config via YAML files
- Environment variables for secrets only (Supabase keys, Stripe keys)
- Docs in Markdown in `/docs/`

## What NOT To Do

- Don't build the dashboard UI yet (MVP is CLI/config only)
- Don't implement SSO/SAML yet (Enterprise tier, Phase 4)
- Don't build Stripe billing yet (Phase 3)
- Don't try to proxy stdio transport yet (HTTP+SSE only for MVP)
- Don't hardcode Supabase/Stripe credentials in source files
