# MCP Zero-Trust Proxy

## What This Is

A drop-in reverse proxy that sits in front of any MCP server and enforces OAuth 2.1 PKCE authentication, tool-level RBAC, session isolation, rate limiting, and immutable audit logging — with zero code changes to the protected server. Built for independent developers, AI-native agencies, and mid-market engineering teams deploying MCP servers.

## Core Value

Any MCP server can be protected with enterprise-grade security in 5 minutes via a single Docker container — no code changes required to the upstream server.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Landing page live at mcpzerotrust.dev collecting waitlist emails
- [ ] OAuth 2.1 PKCE authentication (GitHub + Google providers minimum)
- [ ] Reverse proxy for HTTP+SSE MCP transport
- [ ] 3 built-in RBAC roles (admin, read-only, tool-restricted)
- [ ] Per-client session isolation
- [ ] Structured JSON audit logging (stdout + file)
- [ ] Per-client rate limiting
- [ ] Docker image + standalone binary deployment
- [ ] YAML-based configuration
- [ ] Quick-start documentation

### Out of Scope

- Dashboard web UI — CLI-only for MVP, dashboard is Phase 3+
- Custom RBAC role definitions — Pro tier, Phase 3
- SSO/SAML — Enterprise tier, Phase 4
- SIEM/OpenTelemetry export — Enterprise tier, Phase 4
- Stripe billing integration — Phase 3
- stdio-to-HTTP wrapper — nice-to-have, not blocking MVP

## Context

- **Market timing**: Clawdbot breach (Jan 2026) exposed 1,800+ servers. 8,000+ MCP servers with zero auth found in Feb 2026. 30+ CVEs in 60 days. OWASP, Microsoft, Palo Alto Unit 42 all published emergency MCP security guidance.
- **Competitive gap**: Closest competitor (MCP Auth Proxy by sigbit) is free/drop-in but lacks RBAC, audit logging, session isolation. Enterprise players (Kong, Lunar, MintMCP) have features but are complex and expensive. Nobody checks all six boxes (drop-in + RBAC + session isolation + audit + simple pricing + easy deploy).
- **Composite score**: 8.1/10, ranked #1 of 49 ideas across 10 research reports.
- **Research complete**: Competitive analysis (12 competitors), attack surface analysis, executive report, go-to-market outreach templates all in `/docs/`.
- **Domain**: mcpzerotrust.dev (purchased via Vercel)
- **Landing page**: Built, needs Supabase backend for waitlist form

## Constraints

- **Tech stack**: Go for proxy core (single binary, low latency), Next.js + Tailwind for dashboard (later)
- **Timeline**: 90-day roadmap. Phase 0 (infrastructure) now, Phase 1 (validation) days 1-7, Phase 2 (MVP) days 8-21
- **Solo founder**: Andrew Noble, non-developer — Claude builds everything
- **Auth standard**: OAuth 2.1 PKCE per MCP spec — no proprietary lock-in
- **Deployment model**: Self-hosted first (Docker + binary), SaaS later
- **Pricing**: $49/$99/$199 per month tiers + $299 one-time binary license

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go for proxy core | Single binary, excellent HTTP/reverse-proxy stdlib, low latency | — Pending |
| Self-hosted first | Security buyers prefer data control | — Pending |
| OAuth 2.1 PKCE | MCP spec standard, no proprietary lock-in | — Pending |
| $49/$99/$199 pricing | Middle ground between free OSS and enterprise pricing | — Pending |
| Supabase for waitlist/dashboard DB | Andrew's existing stack, free tier sufficient for pre-launch | — Pending |

---
*Last updated: 2026-03-19 after initialization*
