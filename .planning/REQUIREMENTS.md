# Requirements: MCP Zero-Trust Proxy

**Defined:** 2026-03-19
**Core Value:** Any MCP server can be protected with enterprise-grade security in 5 minutes via a single Docker container — no code changes required.

## v1 Requirements

Requirements for initial release (90-day roadmap). Each maps to roadmap phases.

### Infrastructure

- [ ] **INFRA-01**: Landing page live at mcpzerotrust.dev collecting waitlist emails via Supabase
- [ ] **INFRA-02**: Git repo initialized and pushed to GitHub (private)
- [ ] **INFRA-03**: Supabase project with waitlist table and RLS policies

### Validation

- [ ] **VALID-01**: 15+ pain signals identified from MCP security community
- [ ] **VALID-02**: 8+ substantive replies from direct outreach describing security concerns
- [ ] **VALID-03**: 2 of 3 discovery calls confirm zero/weak auth AND willingness to pay $49+/mo
- [ ] **VALID-04**: 25+ email signups on waitlist
- [ ] **VALID-05**: Go/no-go brief written scoring 5 key questions with evidence

### Proxy Core

- [ ] **PRXY-01**: OAuth 2.1 PKCE authentication flow with token exchange and refresh
- [ ] **PRXY-02**: Transparent reverse proxy for HTTP+SSE MCP transport (no server code changes)
- [ ] **PRXY-03**: Per-client session isolation preventing cross-tenant data leakage
- [ ] **PRXY-04**: Structured JSON audit log of every MCP call (who, what, when, allowed/denied)
- [ ] **PRXY-05**: Single Docker container or standalone binary deployment
- [ ] **PRXY-06**: YAML-based configuration (server URL, allowed clients, token issuer)

### RBAC & Controls

- [ ] **RBAC-01**: 3 built-in RBAC roles (admin, read-only, tool-restricted) per client
- [ ] **RBAC-02**: Tool-level permission enforcement on tools/call and tools/list methods
- [ ] **RBAC-03**: Per-client rate limiting to prevent abuse

### Documentation

- [ ] **DOCS-01**: Quick-start guide for Docker and binary install
- [ ] **DOCS-02**: Configuration reference for YAML config
- [ ] **DOCS-03**: Integration tests against 5 MCP server types

### Beta & Revenue

- [ ] **BETA-01**: 10+ active proxy deployments in production environments
- [ ] **BETA-02**: Stripe billing integration with 3 pricing tiers ($49/$99/$199)
- [ ] **BETA-03**: $500+ MRR from 5+ paying customers
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
| INFRA-01 | Phase 0 | In Progress |
| INFRA-02 | Phase 0 | Complete |
| INFRA-03 | Phase 0 | Complete |
| VALID-01 | Phase 1 | Pending |
| VALID-02 | Phase 1 | Pending |
| VALID-03 | Phase 1 | Pending |
| VALID-04 | Phase 1 | Pending |
| VALID-05 | Phase 1 | Pending |
| PRXY-01 | Phase 2 | Pending |
| PRXY-02 | Phase 2 | Pending |
| PRXY-03 | Phase 2 | Pending |
| PRXY-04 | Phase 2 | Pending |
| PRXY-05 | Phase 2 | Pending |
| PRXY-06 | Phase 2 | Pending |
| RBAC-01 | Phase 2 | Pending |
| RBAC-02 | Phase 2 | Pending |
| RBAC-03 | Phase 2 | Pending |
| DOCS-01 | Phase 2 | Pending |
| DOCS-02 | Phase 2 | Pending |
| DOCS-03 | Phase 2 | Pending |
| BETA-01 | Phase 3 | Pending |
| BETA-02 | Phase 3 | Pending |
| BETA-03 | Phase 3 | Pending |
| BETA-04 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 24 total
- Mapped to phases: 24
- Unmapped: 0

---
*Requirements defined: 2026-03-19*
*Last updated: 2026-03-19 after initialization*
