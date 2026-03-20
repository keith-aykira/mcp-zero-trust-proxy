# RSCH-06: Go/No-Go Brief — MCP Zero-Trust Proxy

**Date:** 2026-03-19 | **Decision required by:** Andrew Noble

---

## Executive Summary

All five kill criteria evaluated. None triggered. **Recommendation: GO.**

The MCP security market has real pain (15+ developer complaints with direct quotes), a growing threat landscape (6 new CVEs in Feb-Mar 2026 alone), no turnkey competitor occupying our lane (simple + enterprise + transparent pricing), strong ecosystem adoption (52M+ monthly PyPI downloads, ~4K+ registered servers, Fortune-scale enterprise adoption), and viable buyer segments willing to pay $49+/mo.

However, the window is narrowing. IBM ContextForge shipped RBAC, Pomerium added MCP support, and PointGuard AI just launched (Mar 18). We need to ship MVP fast.

---

## Kill Criteria Scoring

### 1. Pain is theoretical, not real (fewer than 5 genuine complaints)

| Score | Evidence |
|-------|----------|
| **CLEAR PASS** | 15 genuine developer complaints with direct quotes found across Reddit, HN, and Twitter/X |

**Key evidence:**
- 220K+ exposed MCP instances (Twitter, Mar 2026)
- 312K user breach with RCE (Twitter, Feb 2026)
- "30 CVEs in 60 days, 437K compromised downloads" (HN, Mar 2026)
- "Compliance team says hell no" to MCP deployments (Twitter, Mar 2026)
- Daily auth token failures in production connectors (Reddit, Mar 2026)

**Pain is not just real — it's escalating.** Developers are being actively hacked, not just worried.

---

### 2. A competitor has shipped a turnkey solution that closes our gap

| Score | Evidence |
|-------|----------|
| **PASS (but narrowing)** | No competitor offers drop-in + enterprise features + transparent pricing |

**Competitive landscape:**
- **sigbit (closest):** Drop-in but NO RBAC, audit, session isolation. Free/OSS.
- **IBM ContextForge:** Now has RBAC but requires Redis + K8s. NOT simple.
- **Pomerium:** Added MCP support but general-purpose zero-trust, not MCP-first.
- **Lunar MCPX:** $250/gateway/mo — expensive and SaaS-focused.
- **Kong/Cloudflare:** Enterprise complex, vendor lock-in.
- **PointGuard AI:** Just announced (Mar 18) — no product yet.

**Our lane (simple + enterprise + transparent) remains unoccupied.** But competitors are converging — IBM from the enterprise side, sigbit from the simple side.

---

### 3. MCP spec or Anthropic/OpenAI is adding built-in auth that eliminates the need

| Score | Evidence |
|-------|----------|
| **PASS** | OAuth 2.1 added to spec but optional. No platform shipping enforcement. |

**Key findings:**
- MCP spec now includes OAuth 2.1 — but it's a STANDARD, not a MANDATE. Implementations can still skip it.
- **Anthropic:** No built-in auth gateway announced. Focus on agent evals and Claude Code integration.
- **OpenAI:** No MCP auth announcements in Jan-Mar 2026.
- **Microsoft:** Building auth into Azure managed MCP (Foundry), but only for Azure customers.
- **Google:** MCP Toolbox SDK with auth for GCP, but only for GCP customers.

**The gap between "auth is available" and "auth is enforced" is our product.** Platform vendors are solving auth for THEIR managed offerings, leaving the self-hosted/multi-cloud/open-source long tail unprotected.

---

### 4. MCP adoption is stalling (flat or declining SDK downloads)

| Score | Evidence |
|-------|----------|
| **PASS** | Downloads normalizing from hype peak but at massive absolute volume |

**Download trends:**
- PyPI `mcp` package: Nov 2025 peak (~60M/mo) → Feb 2026 (~52M/mo). Down 13% but still enormous.
- This is expected hype-to-normalization, not a crash.
- Enterprise adoption is ACCELERATING even as raw downloads moderate: Atlassian Rovo GA, PayPal MCP, Entro AGA, SurePath AI.
- 6 AI platforms now support MCP natively. No competing protocol has emerged.
- ~4K+ servers on Smithery, ~10K+ ecosystem-wide estimate.

**MCP is the de facto standard. Adoption is settling into enterprise patterns, which is what we need for a paid security product.**

---

### 5. No viable buyer segment willing to pay $49+/mo

| Score | Evidence |
|-------|----------|
| **PASS** | Multiple segments, clear pricing benchmarks support $49+ |

**Pricing evidence:**
- Solo devs pay $25/mo for security tools (Snyk, Socket — established pattern)
- Small teams pay $25-50/user/mo (industry standard)
- Lunar MCPX charges $250/gateway/mo for team tier — 5x our target
- The gap between free (sigbit) and enterprise (Kong, Cloudflare) is wide
- Compliance triggers (SOC 2, security incidents) force purchase regardless of price sensitivity
- Zero-trust proxy benchmark: $7/user/mo (Pomerium, Cloudflare Access)

**Recommended tiers:** Free → $29/mo (Pro) → $49+$15/seat (Team) → $199/mo (Business) → Custom (Enterprise)

---

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| IBM ContextForge simplifies deployment | Medium | High | Ship before they do. Our Docker one-liner is months ahead of their K8s stack. |
| Anthropic ships built-in auth enforcement | Low (next 6mo) | Fatal | Monitor spec repo weekly. Pivot to audit/RBAC if core auth becomes enforced. |
| PointGuard AI launches before us | Medium | Medium | They just announced — we can ship MVP before they have product-market fit. |
| MCP adoption declines further | Low | High | Download volumes are normalizing, not crashing. Enterprise signals remain strong. |
| Free tier cannibalizes paid | Medium | Medium | Gate SSO, audit logs, and team features behind paid. Standard open-core model. |

---

## Timing Assessment

**The window is open but closing.** Evidence:
- 10+ agentic security startups identified by CRN in 2026
- IBM, Pomerium, Stacklok all adding MCP features rapidly
- PointGuard AI announced Mar 18 — direct competitor positioning
- Enterprise demand is NOW (Atlassian, PayPal, Entro, SurePath)

**Estimated viable window:** 3-6 months to establish position before the market consolidates.

---

## Decision

### GO ✓

**Rationale:**
1. Pain is real and escalating (15+ genuine complaints, active breaches)
2. No turnkey competitor in our lane (simple + enterprise + transparent)
3. Platforms aren't eliminating the need (auth is optional, enforcement is our product)
4. Adoption is massive (52M+ monthly downloads, enterprise acceleration)
5. Viable pricing at $49+/mo (market benchmarks support it)

### Recommended Next Steps

1. **Deploy landing page to Vercel** — connect mcpzerotrust.dev, start collecting waitlist
2. **Phase 2: MVP Build** — Go proxy with OAuth 2.1 PKCE, RBAC (3 roles), audit logging, session isolation
3. **Target:** Ship MVP within 4-6 weeks
4. **First customers:** Solo devs + small teams running open-source MCP servers (self-hosted market)

---

*Research cost: ~$6.86 across 5 Exa deep_researcher_pro queries (85+ pages per query, 420+ total sources)*
