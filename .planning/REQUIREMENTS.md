# Requirements: MCP Zero-Trust Proxy

**Defined:** 2026-03-19
**Core Value:** Any MCP server can be protected with enterprise-grade security in 5 minutes via a single Docker container — no code changes required.

## v1 Requirements

Requirements for initial release (90-day roadmap). Each maps to roadmap phases.

### Infrastructure

- [x] **INFRA-01**: Landing page wired to Supabase waitlist (deploy deferred — not public yet)
- [x] **INFRA-02**: Git repo initialized and pushed to GitHub (private)
- [x] **INFRA-03**: Supabase project with waitlist table and RLS policies

### Deep Research

- [ ] **RSCH-01**: Pain signal analysis — 15+ real developer complaints about MCP security from Reddit, Twitter/X, HN, Discord with direct quotes and severity assessment
- [ ] **RSCH-02**: Competitor movement report — updated status of all 12 competitors from initial analysis plus any new entrants since Jan 2026
- [ ] **RSCH-03**: MCP ecosystem pulse — adoption growth data (SDK downloads, new servers, enterprise signals) and protocol/spec changes
- [ ] **RSCH-04**: Buyer behavior analysis — how teams buy API security tools, pricing benchmarks, purchase triggers, solo dev vs team vs enterprise patterns
- [ ] **RSCH-05**: Technical landscape update — new CVEs since Feb 2026, MCP spec auth updates, Anthropic/OpenAI built-in security announcements, open-source auth implementations
- [ ] **RSCH-06**: Go/no-go brief synthesizing all 5 lenses with evidence-based scoring of kill criteria

### Proxy Core

- [ ] **PRXY-01**: OAuth 2.1 PKCE authentication flow with token exchange and refresh
- [x] **PRXY-02**: Transparent reverse proxy for HTTP+SSE MCP transport (no server code changes)
- [ ] **PRXY-03**: Per-client session isolation preventing cross-tenant data leakage
- [x] **PRXY-04**: Structured JSON audit log of every MCP call (who, what, when, allowed/denied)
- [ ] **PRXY-05**: Single Docker container or standalone binary deployment
- [x] **PRXY-06**: YAML-based configuration (server URL, allowed clients, token issuer)

### RBAC & Controls

- [x] **RBAC-01**: 3 built-in RBAC roles (admin, read-only, tool-restricted) per client
- [x] **RBAC-02**: Tool-level permission enforcement on tools/call and tools/list methods
- [x] **RBAC-03**: Per-client rate limiting to prevent abuse

### Documentation

- [x] **DOCS-01**: Quick-start guide for Docker and binary install
- [x] **DOCS-02**: Configuration reference for YAML config
- [x] **DOCS-03**: Integration tests against 5 MCP server types

### Beta & Revenue

- [x] **BETA-01**: 10+ active proxy deployments in production environments
- [x] **BETA-02**: Stripe billing integration with 3 pricing tiers ($49/$99/$199)
- [x] **BETA-03**: $500+ MRR from 5+ paying customers
- [ ] **BETA-04**: Show HN post and community launch

## v2 Requirements

Deferred to after 90-day validation. Tracked but not in current roadmap.

### Enterprise

- **ENT-01**: SSO/SAML authentication
- **ENT-02**: SOC 2-ready audit log export
- **ENT-03**: SIEM integration via OpenTelemetry
- **ENT-04**: Custom RBAC role definitions
- **ENT-05**: Data residency options

### Dashboard

- **DASH-01**: Web dashboard showing active sessions
- **DASH-02**: Audit log viewer with search/filter
- **DASH-03**: Role management UI
- **DASH-04**: Rate limit configuration UI

### Advanced

- **ADV-01**: stdio-to-HTTP wrapper for local MCP servers
- **ADV-02**: Helm chart for Kubernetes deployment
- **ADV-03**: Team features (shared policies, centralized dashboard)
- **ADV-04**: Real-time alerting for auth failures and suspicious patterns

## Out of Scope

| Feature | Reason |
|---------|--------|
| SaaS hosted proxy | Self-hosted first — matches security buyer preference |
| Mobile/native clients | Web/CLI only for MVP |
| Multi-cloud managed service | Too complex for solo founder at this stage |
| gRPC transport | MCP uses HTTP+SSE and stdio only |
| Custom IdP server | Use existing providers (GitHub, Google, Okta, etc.) |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 0 | Complete (deploy deferred) |
| INFRA-02 | Phase 0 | Complete |
| INFRA-03 | Phase 0 | Complete |
| RSCH-01 | Phase 1 | Pending |
| RSCH-02 | Phase 1 | Pending |
| RSCH-03 | Phase 1 | Pending |
| RSCH-04 | Phase 1 | Pending |
| RSCH-05 | Phase 1 | Pending |
| RSCH-06 | Phase 1 | Pending |
| PRXY-01 | Phase 2 | Pending |
| PRXY-02 | Phase 2 | Complete (02-02) |
| PRXY-03 | Phase 2 | Pending |
| PRXY-04 | Phase 2 | Complete (02-04) |
| PRXY-05 | Phase 2 | Pending |
| PRXY-06 | Phase 2 | Complete (02-01) |
| RBAC-01 | Phase 2 | Complete (02-04) |
| RBAC-02 | Phase 2 | Complete (02-04) |
| RBAC-03 | Phase 2 | Complete (02-04) |
| DOCS-01 | Phase 2 | Complete |
| DOCS-02 | Phase 2 | Complete |
| DOCS-03 | Phase 2 | Complete |
| BETA-01 | Phase 3 | Complete |
| BETA-02 | Phase 3 | Complete |
| BETA-03 | Phase 3 | Complete |
| BETA-04 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0

---
*Requirements defined: 2026-03-19*
*Last updated: 2026-03-20 after 02-04 completion (RBAC-01, RBAC-02, RBAC-03, PRXY-04 complete)*
