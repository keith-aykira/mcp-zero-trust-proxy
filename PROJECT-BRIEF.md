# MCP Zero-Trust Proxy — Product Brief

**Version:** 0.1 | **Date:** March 19, 2026

---

## Problem

MCP (Model Context Protocol) servers are being deployed at scale — 97M+ monthly SDK downloads, 10,000+ active public servers — but authentication is optional in the spec. The result:

- 8,000+ MCP servers exposed with zero authentication (Feb 2026)
- 30+ CVEs in 60 days (Jan–Feb 2026), including a CVSS 9.6 RCE in the most popular OAuth workaround
- Clawdbot breach exposed 1,800+ servers; infostealers targeted them within 48 hours
- 41% of surveyed production servers have no auth whatsoever
- OWASP, Microsoft, and Palo Alto Unit 42 all published emergency MCP security guidance in Feb 2026

Developers know this is a problem but existing solutions are either too simple (MCP Auth Proxy — no RBAC, no audit logging) or too complex (Kong — enterprise pricing, Kubernetes required, weeks to deploy).

## Solution

A reverse proxy that sits in front of any MCP server and adds:

1. **OAuth 2.1 PKCE authentication** — multi-provider (GitHub, Google, Okta, Auth0, Azure AD, Keycloak, any OIDC)
2. **Tool-level RBAC** — control which tools each client can call (admin, read-only, tool-restricted, custom roles)
3. **Session isolation** — per-client boundaries preventing cross-tenant data leakage
4. **Token exchange and scoping** — resource-scoped access tokens for downstream API calls
5. **Immutable audit logging** — every MCP call logged (who, what, when, allowed/denied), SOC 2-ready export, OpenTelemetry integration
6. **Rate limiting** — per-client request throttling

**Zero code changes required to the protected MCP server.** Docker pull, set target and auth provider, done.

## Target Users

1. **Independent developers** running 1–3 MCP servers for personal/client projects (Starter tier)
2. **AI-native agencies** running 3–10 MCP servers for multiple clients (Pro tier)
3. **Mid-market engineering teams** deploying MCP to internal databases and enterprise APIs (Enterprise tier)

## Positioning

> "The only drop-in authentication proxy for MCP servers that combines OAuth 2.1 PKCE with enterprise-grade RBAC, session isolation, and compliance-ready audit logging — all in a 5-minute deployment."

**vs. MCP Auth Proxy (sigbit):** We add RBAC, session isolation, audit logging, rate limiting. They're auth-only.
**vs. Kong:** We deploy in 5 minutes, not 5 days. No Kubernetes required. Transparent pricing.
**vs. Lunar/MintMCP:** We're self-hosted, transparent pricing, no "contact sales" friction.
**vs. nginx reverse proxy:** We understand MCP semantics. Tool-level RBAC, not just IP/path routing.

## Pricing

| Tier | Price | Servers | Key Features |
|------|-------|---------|--------------|
| Starter | $49/mo | 1 | OAuth 2.1, basic RBAC (3 roles), 7-day audit log |
| Pro | $99/mo | 5 | Custom roles, session isolation, rate limiting, 30-day audit log |
| Enterprise | $199/mo | Unlimited | SSO/SAML, SOC 2 export, SIEM integration, 90-day audit log, SLA |
| Binary License | $299 (one-time) | 1 | Self-hosted forever, no SaaS dependency |

Beta: Free Pro access for the first 50 teams.

## Architecture (Recommended)

```
┌─────────────┐     ┌──────────────────────────┐     ┌────────────────┐
│  MCP Client  │────▶│  MCP Zero-Trust Proxy     │────▶│  MCP Server    │
│  (Claude,    │     │                            │     │  (any server,  │
│   Cursor,    │     │  ┌─────────┐ ┌──────────┐ │     │   unchanged)   │
│   Copilot)   │◀────│  │  Auth   │ │  RBAC    │ │◀────│                │
│              │     │  │  Layer  │ │  Engine  │ │     │                │
└─────────────┘     │  └─────────┘ └──────────┘ │     └────────────────┘
                    │  ┌─────────┐ ┌──────────┐ │
                    │  │  Rate   │ │  Audit   │ │
                    │  │  Limit  │ │  Logger  │ │
                    │  └─────────┘ └──────────┘ │
                    │  ┌─────────┐              │
                    │  │ Session │              │
                    │  │ Isolator│              │
                    │  └─────────┘              │
                    └──────────────────────────┘
                              │
                    ┌─────────▼──────────┐
                    │  Dashboard (Web UI) │
                    │  - Active sessions  │
                    │  - Audit log viewer │
                    │  - Role management  │
                    │  - Rate limit config│
                    └────────────────────┘
```

### Tech Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Proxy core | Go | Single binary, low latency, excellent HTTP/reverse-proxy stdlib |
| Auth | Built-in OAuth 2.1 library | No external IdP dependency for Starter tier |
| Dashboard | Next.js + Tailwind | Andrew's existing stack |
| Database | SQLite (single-node) / Postgres (cloud) | SQLite for simplicity, Postgres for teams |
| Billing | Stripe | Andrew's existing integration experience |
| Config | YAML | Simple, well-understood, version-controllable |
| Deployment | Docker + binary + Helm (optional) | Cover all deployment preferences |
| CI/CD | GitHub Actions | Standard, free for open source |
| Telemetry | OpenTelemetry | Industry standard, SIEM-compatible |

### MCP Protocol Details

The proxy must understand MCP's transport layers:

- **HTTP+SSE (Streamable HTTP)** — Primary remote transport. Proxy intercepts HTTP requests, validates auth, forwards to upstream.
- **stdio** — Local transport (process-level). Not directly proxyable — the proxy handles the network boundary so upstream stdio servers get wrapped in an HTTP layer.
- **JSON-RPC 2.0** — Message format. Proxy must parse method names to enforce tool-level RBAC (e.g., allow `tools/call` for `read_file` but deny `tools/call` for `execute_command`).

Key MCP methods to intercept:
- `initialize` — session setup, capability negotiation
- `tools/list` — filter visible tools per RBAC role
- `tools/call` — enforce per-tool permissions
- `resources/read` — enforce resource access policies
- `prompts/get` — optional: filter prompt templates

### MVP Scope (Days 8-21)

**In scope:**
- OAuth 2.1 PKCE auth flow (GitHub + Google providers minimum)
- Reverse proxy for HTTP+SSE MCP transport
- 3 built-in RBAC roles (admin, read-only, tool-restricted)
- Per-client session isolation
- Structured JSON audit log (stdout + file)
- Per-client rate limiting
- Docker image + standalone binary
- YAML config
- Quick-start docs

**Out of scope for MVP:**
- Dashboard web UI (CLI-only for MVP)
- Custom RBAC role definitions (Pro tier, Phase 3)
- SSO/SAML (Enterprise tier, Phase 4)
- SIEM/OpenTelemetry export (Enterprise tier, Phase 4)
- Stripe billing integration (Phase 3)
- stdio-to-HTTP wrapper (nice-to-have, not blocking)

## Competitive Landscape Summary

See `docs/research/COMPETITIVE-ANALYSIS.md` for the full analysis. Key gaps we exploit:

| Gap | Who Has It | Who Doesn't | We Will |
|-----|-----------|-------------|---------|
| Drop-in (no code changes) | MCP Auth Proxy | Kong, Lunar, Cerbos | ✓ |
| Tool-level RBAC | Kong, Lunar, Cerbos | MCP Auth Proxy | ✓ |
| Session isolation | Pomerium | Everyone else | ✓ |
| Audit logging | Kong, Lunar, MintMCP | MCP Auth Proxy, Cerbos | ✓ |
| Simple pricing | MCP Auth Proxy (free) | Kong, Lunar, MintMCP | ✓ |
| Easy deployment | MCP Auth Proxy | Kong, IBM ContextForge | ✓ |

Nobody checks all six boxes. We will.

## Success Metrics (90-Day)

| Metric | Target | Source |
|--------|--------|--------|
| Active proxy deployments | 100+ | Telemetry |
| Paying teams | 10+ | Stripe |
| MRR | $2,000+ | Stripe |
| Monthly churn | < 10% | Stripe |
| Security incidents caught | 1+ documented | Audit logs |
| Unsolicited referrals | 3+ | Outreach tracking |
| Waitlist signups | 50+ (before launch) | Supabase |
