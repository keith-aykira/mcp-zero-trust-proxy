# Go/No-Go Decision Brief — MCP Zero-Trust Proxy

**Date:** 2026-03-19
**Phase:** 1 — Deep Research Synthesis
**Decision maker:** Andrew Noble
**Prepared by:** Claude (AI research assistant)

---

## Bottom Line Up Front

**Recommendation: CONDITIONAL GO**

All 5 kill criteria pass, but 2 carry CAUTION flags: (1) SDK downloads are normalizing from peak, not accelerating, and (2) the competitive window is 3-6 months and narrowing. The strongest evidence is the pain — 14 genuine developer complaints, 86% at "blocking" or "dangerous" severity, with active breaches and 220K+ exposed instances. Nobody occupies the "simple + enterprise + transparent" lane yet.

Proceed to MVP build, but ship fast. The window closes with every passing month.

---

## Kill Criteria Scorecard

| # | Kill Criterion | Status | Confidence | Key Evidence |
|---|---|---|---|---|
| 1 | Pain is theoretical (< 5 genuine complaints) | **PASS** | HIGH | 14 complaints found, 86% blocking/dangerous, active breaches |
| 2 | Competitor shipped turnkey solution | **CAUTION** | HIGH | No turnkey competitor in our lane, but IBM shipped RBAC + 4 new entrants in 90 days |
| 3 | MCP spec adding built-in auth | **PASS** | HIGH | OAuth 2.1 in spec but optional. No platform shipping enforcement. |
| 4 | MCP adoption stalling | **CAUTION** | MEDIUM | Downloads down 13% from Nov peak. Still massive (52M/mo). Enterprise accelerating. |
| 5 | No buyer segment willing to pay $49+/mo | **PASS** | MEDIUM | Comparable tools at $25-50/dev/mo. Lunar charges $250/gateway. But no MCP-specific WTP survey exists. |

**Overall: 3 PASS, 2 CAUTION, 0 FAIL → CONDITIONAL GO**

---

## Detailed Evidence by Criterion

### Criterion 1: Pain is Theoretical

**Status: PASS | Confidence: HIGH**

- Pain signals found: 14 (threshold: 5+)
- Severity breakdown: 8 dangerous (57%), 4 blocking (29%), 2 annoying (14%)
- Key quotes:
  1. "220,000+ OpenClaw instances are exposed to the public internet. Many without authentication." — @hqmank, Twitter, Mar 2026
  2. "Compliance team says 'hell no'" — @OranAITech, Twitter, Mar 2026
  3. "MCP removed integration friction fast but it also merged tool access data access and decision authority before most teams defined ownership or risk boundaries." — r/AI_Agents, Jan 2026
- Workarounds observed: nginx+Authelia, Cloudflare Access, bearer tokens in config, mcp-remote OAuth bridge, Azure APIM, prmichaelsen/mcp-auth wrapper, manual disconnect/reconnect
- **Assessment:** Pain is real, escalating, and driving DIY workarounds. Every workaround is either too simple (no RBAC/audit) or too complex (requires K8s/Azure). Our "simple middle ground" has clear demand.

### Criterion 2: Competitor Shipped Turnkey Solution

**Status: CAUTION | Confidence: HIGH**

- Closest competitor: IBM ContextForge — now has RBAC (shipped in v1.0.0-RC2, Mar 9) but requires Redis + K8s
- Gap analysis: No competitor checks ALL boxes (drop-in + RBAC + audit + session isolation + self-hosted + transparent pricing)
- All 12 original competitors verified + 4 new entrants (PointGuard AI, Salt Security, prmichaelsen/mcp-auth, AthenZ/mcp-oauth-proxy)
- Time window estimate: 3-6 months before market consolidates
- **Why CAUTION, not PASS:** The rate of competitor entry is accelerating. IBM shipped RBAC in one release cycle. Pomerium added full MCP support. PointGuard AI launched with direct competitor positioning (Mar 18). We're one product cycle away from someone else shipping this.

### Criterion 3: MCP Spec Adding Built-in Auth

**Status: PASS | Confidence: HIGH**

- Current spec auth status: OAuth 2.1 added as a standard (not mandate). Implementations can skip it.
- Planned changes: SMCP (Secure MCP) proposals exist as community documents, not merged into official spec
- Platform announcements: Microsoft and Google building auth into THEIR managed offerings only. Anthropic/OpenAI have NOT announced built-in auth gateways.
- Timeline risk: No evidence of mandatory spec auth in next 6-12 months. Even if added, enforcement + RBAC + audit + session isolation remain beyond spec scope.
- **Assessment:** The spec standardizes auth flows but doesn't enforce them. Enforcement is our product. Platform vendors are solving for their own customers, leaving the multi-cloud/self-hosted/OSS long tail open.

### Criterion 4: MCP Adoption Stalling

**Status: CAUTION | Confidence: MEDIUM**

- SDK downloads: PyPI `mcp` package — Nov 2025 peak (~60M/mo) → Feb 2026 (~52M/mo) = **-13%**
- Server count: ~4K on Smithery, ~10K+ ecosystem-wide
- Platform support: 6 major platforms (Claude, ChatGPT, Copilot, Cursor, Windsurf, n8n)
- Enterprise signals: Atlassian Rovo GA, PayPal MCP, Entro AGA, SurePath AI
- **Why CAUTION, not PASS:** Downloads are normalizing, not crashing, but we don't have evidence of re-acceleration. The 97M/mo figure from earlier reports may have included aggregate SDK + server packages. Enterprise adoption signals are strong qualitatively but lack hard install counts.
- **Why not FAIL:** 52M monthly downloads is enormous absolute volume. No competing protocol exists. Enterprise adoption is accelerating even as hype-phase downloads moderate.

### Criterion 5: No Buyer Segment Willing to Pay $49+/mo

**Status: PASS | Confidence: MEDIUM**

- Comparable tool pricing: $25-50/dev/mo (Snyk, Socket, Semgrep); $7/user/mo (zero-trust proxies)
- Our pricing vs market: $49/mo is below Lunar ($250/gateway) and in line with developer security tool norms
- Most viable segment: Small teams (2-20 devs) with compliance pressure
- Purchase triggers: compliance audit, security incident, customer requirement, insurance
- Revenue path: 10-18 paying customers needed for $500 MRR target
- **Why MEDIUM confidence, not HIGH:** No direct survey of "would you pay for MCP security specifically?" exists. We're extrapolating from adjacent categories (API security, zero-trust proxies, dev security tools). The $25/mo indie benchmark and $250/gateway Lunar ceiling bracket our price well, but nobody has proven MCP-specific paid demand yet.

---

## Strengths (Reasons to Proceed)

1. **Pain is acute and worsening** — active breaches, 220K+ exposed servers, 30+ CVEs, compliance teams blocking adoption
2. **Clear product gap** — nobody has simple + enterprise + transparent pricing in one product
3. **Standard without enforcement** — OAuth 2.1 in spec but optional = our product fills the enforcement gap
4. **Platform fragmentation** — Microsoft/Google solving for their own clouds leaves multi-cloud/OSS market open
5. **Workaround proliferation** — nginx+Authelia, Azure APIM, mcp-auth wrapper all validate demand for exactly what we're building

---

## Risks (Reasons for Caution)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| IBM ContextForge simplifies to single Docker command | Medium | High | Ship before they do. Our MVP target is 4-6 weeks. |
| Anthropic ships mandatory auth enforcement in spec | Low (next 6mo) | Fatal | Monitor spec repo weekly. Pivot to audit/RBAC if core auth becomes enforced. |
| PointGuard AI launches a product before us | Medium | Medium | They announced Mar 18 with no product. We can ship first. |
| MCP adoption continues declining | Low | High | Enterprise adoption accelerating compensates for hype-phase download normalization. |
| Free OSS (sigbit, mcp-auth) is "good enough" | Medium | Medium | Gate RBAC, audit, session isolation, SSO behind paid tiers. |
| Solo founder capacity — can we actually ship in 4-6 weeks? | Medium | High | Scope MVP tightly. Go binary + YAML config. No dashboard UI in v1. |

---

## If GO: Recommended Next Steps

1. **Deploy landing page to Vercel** — connect mcpzerotrust.dev, start collecting waitlist signups now
2. **Phase 2: MVP Build** — Go proxy: OAuth 2.1 PKCE, 3 built-in RBAC roles, JSON audit log, session isolation
3. **Target:** Ship MVP within 4-6 weeks
4. **First customers:** Solo devs + small teams running open-source MCP servers (self-hosted market)
5. **Pricing:** Launch with Free + Pro ($29/mo) + Team ($49/mo) tiers only. Add Business/Enterprise after first revenue.

## If PIVOT: Recommended Direction

If evidence changes (e.g., mandatory auth in spec, IBM ships simple deploy):
- **Pivot A:** Narrow to audit/compliance layer only (RBAC + audit logging + compliance reports). Auth becomes table stakes, enforcement/compliance remains paid.
- **Pivot B:** Shift to managed gateway-as-a-service (like Lunar/Cloudflare) instead of self-hosted proxy. Higher margin, different buyer.
- **Pivot C:** Open-source the proxy, monetize via hosted dashboard + analytics (Grafana model).

## If KILL: Recommended Actions

If 2+ kill criteria flip to FAIL:
1. Take landing page offline
2. Write post-mortem documenting what was learned
3. Redirect energy to AutoTrader ETF or AutoFoundry (both have active revenue paths)
4. Keep research docs — competitive analysis has standalone value

---

## Research Confidence Assessment

| Report | Data Quality | Coverage | Confidence |
|--------|-------------|----------|------------|
| Pain Signals | HIGH (direct quotes with URLs) | 14 signals across 3 platforms | **HIGH** |
| Competitive Analysis | HIGH (GitHub stats, release notes, pricing pages) | 12/12 original + 4 new entrants | **HIGH** |
| MCP Ecosystem Pulse | MEDIUM (PyPI stats reliable, npm needs manual aggregation) | SDK downloads + server counts + enterprise signals | **MEDIUM** |
| Buyer Behavior | MEDIUM (pricing from vendor pages, WTP extrapolated from adjacent markets) | 9 tool benchmarks, 4 segments, 6 triggers | **MEDIUM** |
| Technical Landscape | HIGH (CVEs from NVD, platform announcements from official docs) | 6 CVEs, 4 platform vendors, OWASP/NIST/SOC2 | **HIGH** |

**Overall research confidence: MEDIUM-HIGH.** Pain and competition evidence is strong. Buyer WTP and ecosystem trajectory have some extrapolation from adjacent markets.

---

## Appendix: Source Reports

- [Pain Signals](PAIN-SIGNALS.md)
- [Competitive Analysis](COMPETITIVE-ANALYSIS.md)
- [MCP Ecosystem Pulse](MCP-ECOSYSTEM-PULSE.md)
- [Buyer Behavior](BUYER-BEHAVIOR.md)
- [Technical Landscape](TECHNICAL-LANDSCAPE.md)
- [Attack Surface](ATTACK-SURFACE.md) (baseline)

---

*Research cost: ~$7.50 across 5 Exa deep_researcher_pro queries + 4 targeted Exa searches (85+ pages per deep query, 420+ total sources)*
