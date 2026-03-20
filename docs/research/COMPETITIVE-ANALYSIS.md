# MCP Zero-Trust Proxy — Competitive Analysis

**Date:** March 19, 2026 | **Classification:** Confidential

---

## Executive Summary

The MCP security landscape is rapidly evolving but highly fragmented. MCP shipped with optional authentication, creating a critical market gap: 41–59% of production servers have no authentication, and 7,000–42,665 exposed servers exist on the public internet. Multiple point solutions have emerged, but the market lacks a unified, simple drop-in zero-trust proxy with PKCE, token exchange, session isolation, RBAC, and audit logging integrated into a single product.

---

## Direct Competitors: MCP Authentication Proxies

### MCP Auth Proxy (sigbit) — CLOSEST COMPETITOR

**What it does:** Drop-in OAuth 2.1/OIDC gateway. No code changes required. Multi-provider support (Google, GitHub, Okta, Auth0, Azure AD, Keycloak, passwordless). Verified across Claude, Claude Code, ChatGPT, GitHub Copilot, Cursor.

**What it doesn't do:** No built-in RBAC or tool-level access control. No session isolation or multi-tenant support. No audit logging or compliance features. No token exchange or resource-scoped access tokens. Limited observability.

**Pricing:** Free / open source

**Key gaps to exploit:** Missing granular access control (tool-level RBAC), session isolation, audit trail for compliance, and token scoping.

### mcp-oauth-gateway (atrawog)

**What it does:** OAuth 2.1 Authorization Server, GitHub as identity provider by default, no code modification needed.

**What it doesn't do:** Single identity provider (GitHub only). No RBAC, session isolation, or audit logging. Limited enterprise flexibility.

**Pricing:** Free / open source

### oauth-mcp-proxy (tuannvm)

**What it does:** OAuth 2.1 library for Go MCP servers. One-line integration via `WithOAuth()`.

**What it doesn't do:** Go-only. Not a proxy — requires SDK integration and code changes. No RBAC, session isolation, or audit logging.

**Pricing:** Free / open source

### open-mcp-auth-proxy (WSO2) — DEPRECATED

**Status:** Archived. Confirms market demand but space needs fresh thinking.

---

## Broader Gateway/Proxy Competitors

### Kong AI/MCP Gateway

**What it does:** MCP ↔ HTTP bridging plugin, OAuth 2.1 enforcement, MCP traffic observability.

**What it doesn't do:** Complex enterprise deployment (requires Kong Gateway 3.12+). Expensive and heavyweight. Not drop-in — requires Kong infrastructure. Session isolation and RBAC are add-on plugins.

**Pricing:** Enterprise licensing. High cost of entry.

**Key gaps:** Overkill for auth-only use cases. High cost barrier. Requires Kubernetes/Docker expertise. Multiple plugins needed.

### Cloudflare MCP Server Portals

**What it does:** Centralized MCP gateway, OAuth integration, AI Gateway (observability, rate limiting, caching). Runs on Cloudflare infrastructure.

**What it doesn't do:** Hosted-only (no self-hosted). No RBAC or tool-level access control. Primarily targets Cloudflare-hosted servers. Opaque audit logging.

**Pricing:** Integrated into Cloudflare AI Gateway (SaaS pricing).

**Key gaps:** No on-premise option. Missing fine-grained access control. Vendor lock-in. Customer doesn't control audit trail.

### IBM ContextForge

**What it does:** AI Gateway + registry + proxy for MCP/A2A/REST/gRPC. Built-in OAuth, multi-transport support, admin UI, OpenTelemetry, 40+ plugins, Redis-backed caching.

**What it doesn't do:** Complex deployment (Docker, Redis, optionally Kubernetes). Tool-level RBAC still in development (GitHub Issue #283). Session isolation not explicit. Steep learning curve.

**Pricing:** Free / open source. **GitHub stars:** 472 stars, 68 forks.

**Key gaps:** RBAC still under development. Multi-tenancy incomplete. Requires Redis/Kubernetes knowledge.

### Lunar.dev MCPX

**What it does:** Tool-level RBAC (global, service, tool-level ACLs). Tool description rewriting. Immutable audit trails. 4ms p99 latency. SIEM integration.

**What it doesn't do:** Pricing undisclosed. No free tier. SaaS-only (no self-hosted).

**Key gaps:** Pricing opacity. SaaS-only. Limited public proof points.

### MintMCP

**What it does:** SOC 2 Type II certified gateway. Virtual MCP Servers (role-scoped tool sets). One-click STDIO deployment with OAuth. Audit trails for compliance.

**What it doesn't do:** Enterprise-focused pricing. Complex for simple use cases.

**Key gaps:** High cost barrier for non-regulated orgs. Pricing opacity.

### Pomerium

**What it does:** Zero-Trust proxy with granular per-request authorization, JWT identity, full audit logging, per-tool control.

**What it doesn't do:** MCP support is experimental and unclear maturity level.

**Key gaps:** MCP support immature. Lacks MCP-first messaging.

### Stacklok (ToolHive)

**What it does:** Embedded authz server, tool-level policy (vMCP gateway), Kubernetes RBAC auto-provisioning, OTel telemetry.

**What it doesn't do:** Kubernetes-only. Not suitable for simple deployments.

**Key gaps:** Kubernetes-only excludes simple deployments.

### Cerbos

**What it does:** Policy engine for fine-grained authorization. MCP demo available. OPA-like policy-as-code.

**What it doesn't do:** Not a proxy — requires SDK integration. Authentication is external. Requires policy expertise. No audit logging or session isolation built-in.

**Key gaps:** Doesn't authenticate. Policy-as-code overkill for simple RBAC.

---

## Scanning/Discovery Tools (Not Authentication)

### Proximity
Scanner only. Identifies tools, prompts, resources. NOVA-powered security evaluation. Does not enforce security at runtime. Complements auth solutions.

### MCPSafetyScanner
Academic/research MCP safety auditing. Role emulation (Auditor, Hacker personas). Scanning/assessment only. Limited production use.

### Salt Security MCP Server
API security analysis with AI. Salt MCP Finder for discovery. AWS WAF integration. Not authentication middleware — discovery/threat detection only.

---

## Feature Comparison Matrix

| Feature | MCP Auth Proxy | Kong | Lunar | MintMCP | Pomerium | IBM CF | Cerbos | **Us (Target)** |
|---------|---|---|---|---|---|---|---|---|
| Drop-in proxy | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **✓** |
| OAuth 2.1 PKCE | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | ✗ | **✓** |
| Tool-level RBAC | ✗ | ✓ | ✓ | ~ | ✓ | ✓ | ✓ | **✓** |
| Session isolation | ✗ | ✗ | ✗ | ✗ | ✓ | ~ | ✗ | **✓** |
| Token exchange | ✗ | ✗ | ✗ | ✗ | ~ | ✓ | ✗ | **✓** |
| Audit logging | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | **✓** |
| Self-hosted | ✓ | ~ | ✗ | ~ | ✓ | ✓ | ✓ | **✓** |
| Simple pricing | ✓ | ✗ | ✗ | ✗ | ~ | ✓ | ✓ | **✓** |
| Easy deployment | ✓ | ✗ | ~ | ✗ | ~ | ✗ | ~ | **✓** |

**Our positioning:** The only product that checks every box. Drop-in simplicity of MCP Auth Proxy + enterprise features of Kong/Lunar/MintMCP + transparent pricing.

---

## Key Insight

The closest competitor (MCP Auth Proxy by sigbit) is free and drop-in but lacks RBAC, audit logging, and session isolation. The enterprise competitors (Kong, Lunar, MintMCP) have those features but are complex, expensive, and not drop-in. Nobody occupies the middle ground: **simple deployment + enterprise security features + transparent pricing**. That's our lane.

---

## Sources

- Proximity: github.com/fr0gger/proximity
- MCP Auth Proxy: sigbit.github.io/mcp-auth-proxy/
- Kong MCP Gateway: konghq.com/blog/product-releases/enterprise-mcp-gateway
- Cloudflare MCP Portals: blog.cloudflare.com/zero-trust-mcp-server-portals/
- IBM ContextForge: github.com/IBM/mcp-context-forge
- Pomerium: pomerium.com/blog/secure-access-for-mcp
- Cerbos: cerbos.dev/blog/mcp-authorization
- Salt Security: salt.security/blog/introducing-the-salt-mcp-server
- Lunar MCPX: lunar.dev
- MintMCP: mintmcp.com/blog/enterprise-ai-infrastructure-mcp
- Stacklok ToolHive: stacklok.com

---

## Update: 2026-03-19

**Methodology:** Exa deep_researcher_pro + targeted Exa advanced search.

### Competitor Status Updates

| Competitor | Status | Key Changes Since Jan 2026 | Threat Level |
|---|---|---|---|
| MCP Auth Proxy (sigbit) | Active | v2.5.4 bug fixes only. ~78 stars. | Low (no enterprise features) |
| mcp-oauth-gateway (atrawog) | Dormant | No updates found | None |
| oauth-mcp-proxy (tuannvm) | Active | 21 stars, last push Mar 5. Still a library, not proxy. | Low |
| open-mcp-auth-proxy (WSO2) | Dead | Archived Feb 2, 2026 | None |
| Kong AI/MCP Gateway | Active | MCP Registry, Tool ACLs, AI cost governance | Low (too complex/expensive) |
| Cloudflare MCP Portals | Active | OAuth, observability, auto-sync | Low (vendor lock-in) |
| IBM ContextForge | Active | **v1.0.0-RC2, RBAC SHIPPED (Issue #283)**, ~3.4K stars | **Medium** (closing gap) |
| Lunar.dev MCPX | Active | **Pricing revealed: $250/gateway/mo**, Dynamic Tool Discovery | Medium (but expensive) |
| MintMCP | Active | Enterprise Governance Platform, SOC 2 Type II | Low (enterprise only) |
| Pomerium | Active | **MCP support added** with PKCE, ~4.6K stars | Medium (general-purpose) |
| Stacklok/ToolHive | Active | v0.12.4, expanded beyond K8s | Low-Medium |
| Cerbos | Active | v0.51.0, MCP integration demos | Low (policy engine, not proxy) |

### New Entrants (Since Jan 2026)

| Competitor | What | Threat Level |
|---|---|---|
| PointGuard AI | MCP Security Gateway (announced Mar 18, 2026) | Medium (direct competitor positioning, but no product yet) |
| Salt Security | Agentic Security Platform for AI stack | Low (broader scope, enterprise) |
| prmichaelsen/mcp-auth | OSS TypeScript auth wrapper, zero-modification | Low-Medium (validates our thesis but library not proxy) |
| AthenZ/mcp-oauth-proxy | Java enterprise MCP OAuth proxy, 0 stars | Low (very early) |

### Updated Feature Matrix

| Feature | sigbit | Kong | CF | IBM CF | Lunar | MintMCP | Pomerium | Stacklok | Cerbos | **Us** |
|---------|--------|------|----|--------|-------|---------|----------|----------|--------|--------|
| Drop-in (1 cmd) | Y | N | N | N | N | N | N | N | N | **Y** |
| OAuth 2.1 PKCE | Y | Y | Y | Y | ~ | Y | Y | Y | N | **Y** |
| Tool-level RBAC | N | Y | N | **Y** | Y | ~ | Y | Y | Y | **Y** |
| Session isolation | N | N | N | ~ | N | N | Y | N | N | **Y** |
| Audit logging | N | Y | Y | Y | Y | Y | Y | Y | N | **Y** |
| Self-hosted | Y | ~ | N | Y | Y | ~ | Y | Y | Y | **Y** |
| Transparent pricing | Y(free) | N | N | Y(free) | **Y** | N | Y | Y(free) | Y | **Y** |
| Simple deploy | Y | N | ~ | N | ~ | N | ~ | ~ | ~ | **Y** |

### Other Notable Activity
- 10+ agentic security startups identified by CRN: 7AI, Dropzone AI, Furl, Noma Security, Operant AI, Prophet Security, Reach Security, Simbian, WitnessAI
- cyproxio/mcp-for-security: OSS pentesting tools wrapped as MCP servers (complementary, not competitive)

### Kill Criteria Assessment
**"A competitor has shipped a turnkey solution that closes our gap"**
- **Finding: PASS (with CAUTION)** — No single competitor checks all boxes (drop-in + RBAC + audit + session isolation + self-hosted + transparent pricing). IBM ContextForge is closest but requires Redis + K8s. Window is narrowing: 3-6 months.
- **Confidence: HIGH**
