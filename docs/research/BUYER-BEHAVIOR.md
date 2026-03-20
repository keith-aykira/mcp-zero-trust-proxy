# Buyer Behavior Analysis

**Research date:** 2026-03-19 | **Source:** Exa deep_researcher_pro

---

## 1. Pricing Benchmarks

### API Gateways
| Product | Entry Price | Mid-tier | Enterprise |
|---------|------------|----------|------------|
| Kong Konnect | $25/mo (serverless CP) | $200-500/mo (hybrid/dedicated CP) | Custom |
| Tyk | Free (OSS) | Core/Professional tiers | Custom |
| Apigee | $20/1M calls | $365-$3,431/mo per region | Multi-year bundles |

### Zero-Trust Proxies
| Product | Entry Price | Team | Enterprise |
|---------|------------|------|------------|
| Pomerium | Free (OSS) | $7/user/mo | Custom |
| Tailscale | Free (personal) | $6/user/mo | $18/user/mo+ |
| Cloudflare Access | Free (small) | ~$7/user/mo | Custom (SASE) |

### Developer Security Tools
| Product | Entry Price | Team | Enterprise |
|---------|------------|------|------------|
| Snyk | Free (limited) | $25/dev/mo | Custom |
| Socket | Free (1K scans) | $25/dev/mo | $50/dev/mo+ |
| Semgrep | Free (10 devs) | ~$30/dev/mo | Custom |

### MCP-Specific Competitors
| Product | Entry Price | Team | Enterprise |
|---------|------------|------|------------|
| MCP Auth Proxy (sigbit) | Free | Free | Free |
| Lunar MCPX | Free | $250/gateway/mo | Custom |
| Cerbos Hub | Free (PoC) | $25/mo (dev) | ~$933/mo (prod) |

---

## 2. Purchase Triggers

What makes teams buy vs. build or use free alternatives:

1. **Compliance audit** — SOC 2, PCI DSS, HIPAA, GDPR force central controls (SSO, audit trails, log retention)
2. **Security incident** — An API breach or high-severity finding accelerates purchase decisions
3. **Customer/partner requirement** — Enterprise contracts stipulate specific security controls
4. **Insurance requirement** — Cyber insurance demands demonstrable controls for coverage/premium reduction
5. **Operational cost exceeds license cost** — When DIY auth consistency costs more than purchasing
6. **Time-to-market** — Commercial product saves weeks/months of engineering time

---

## 3. Buyer Segments

### Solo Developers / Indie Hackers
- **Decision maker:** Sole founder (unilateral, fast)
- **Price sensitivity:** Very high. $0 required for trial. $20-30/mo acceptable for clear value.
- **Feature needs:** Lightweight auth, simple SDK, clear docs, minimal infra
- **Evaluation:** 1-7 day trial, quick POC
- **Key insight:** $25/mo is the repeatedly cited "acceptable indie price point"

### Small Teams (2-20 devs)
- **Decision maker:** Engineering manager / tech lead
- **Price sensitivity:** Low-mid double digits per seat
- **Feature needs:** Multi-dev accounts, basic RBAC, GitHub integration, simple SSO, logs
- **Evaluation:** 1-3 week POC, product-led conversion
- **Acceptable range:** $25-50/user/mo

### Mid-Market (20-200 devs)
- **Decision maker:** Engineering leadership, security owners, procurement (5-10 stakeholders)
- **Budget cycles:** Annual planning with quarterly review
- **Feature needs:** SSO/SCIM, RBAC, audit logs, 1yr retention, multi-region, compliance
- **Evaluation:** Formal POC, security assessment, 4-12 week procurement
- **Acceptable range:** $1K-$10K/mo

### Enterprise (200+ devs)
- **Decision maker:** Cross-functional committee (CTO/CISO, procurement, legal, finance)
- **Process:** RFI → RFP → technical eval → legal → procurement (8-26+ weeks)
- **Requirements:** SOC 2/ISO27001/GDPR mapping, SSO, long retention, zero-trust architecture
- **Acceptable range:** Multi-$100K+/year

---

## 4. Willingness to Pay for MCP Security

Direct survey data for "how much would you pay for MCP proxy?" is scarce. Signals:

- **$25/mo** repeatedly appears as acceptable indie/solo price point (Indie Hackers community)
- **$7/user/mo** is the zero-trust proxy benchmark (Pomerium, Cloudflare Access)
- **$250/gateway/mo** (Lunar MCPX Team tier) — expensive for small teams but establishes ceiling
- **Forum discussions** show WTP when: tool saves ops time, prevents incidents, scales predictably

---

## 5. Open Source vs. Paid Boundary

### What developers expect FREE
- Basic request routing
- Core auth/authz primitives (JWT/OAuth validation)
- Rate limiting
- Basic plugins
- Self-hosted deployment

### What unlocks PAID conversion
- **SSO / Enterprise IdP** (SAML, SCIM) — #1 enterprise conversion driver
- **Audit logging with long retention** — compliance evidence
- **High availability / multi-region** with SLAs
- **Advanced analytics and telemetry**
- **Centralized policy management** and multi-tenant capabilities
- **Professional services and dedicated support**
- **Managed/hosted options** (removes operator burden)

---

## 6. Recommended Pricing Structure

Based on market evidence:

| Tier | Price | Target | Features |
|------|-------|--------|----------|
| **Free / OSS** | $0 | Solo devs, evaluation | Core auth, basic rate limiting, community support, self-hosted |
| **Pro** | $29/mo | Indie / solo | 30-day log retention, 50K monthly calls, email support |
| **Team** | $49/mo + $15/seat | Small teams (2-20) | Team seats, basic SSO, 90-day logs, GitHub/Slack integrations |
| **Business** | $199/mo | Mid-market | SSO/SCIM, RBAC, 1yr audit logs, multi-region, priority support |
| **Enterprise** | Custom | 200+ devs | Managed deployment, 99.99% SLA, unlimited retention, dedicated TAM |

**Key insight:** $49/mo base for teams undercuts Lunar ($250/gateway) by 5x while offering comparable features. Free tier is essential for adoption flywheel.

---

## 7. Revenue Path to $500 MRR (Phase 3 Target)

| Scenario | Tier Mix | Customers Needed | Monthly Revenue |
|----------|----------|-----------------|-----------------|
| **Conservative** | 15 Pro ($29) + 3 Team ($49) | 18 customers | $582 MRR |
| **Mid** | 8 Pro ($29) + 5 Team ($49) + 1 Business ($199) | 14 customers | $676 MRR |
| **Aggressive** | 5 Pro ($29) + 3 Team ($49) + 2 Business ($199) | 10 customers | $690 MRR |

**Key math:** We need ~10-18 paying customers to hit $500 MRR. Given ~4,000 registered MCP servers on Smithery alone, converting 0.25-0.45% to paid would hit the target.

**MCP monetization context:** The broader MCP ecosystem has 16,000+ servers but <0.5% earn $1,000+/mo and 95%+ generate zero revenue (DEV Community, Feb 2026). We're not asking MCP servers to monetize — we're selling security to the teams RUNNING those servers. Different buyer, different motion.

---

## Kill Criteria Assessment

**"No viable buyer segment willing to pay $49+/mo"**

**VERDICT: VIABLE.** Market evidence shows:
- Solo devs pay $25/mo for security tools (Snyk, Socket)
- Small teams pay $25-50/user/mo (standard in the space)
- Compliance triggers force purchase regardless of price sensitivity
- Lunar charges $250/gateway/mo for team tier — our $49 is well below
- The gap between free (sigbit) and enterprise (Kong, Cloudflare, Lunar) is WIDE — our mid-market positioning fills it

---

*Sources: Kong, Tyk, Apigee, Pomerium, Tailscale, Cloudflare, Snyk, Socket, Semgrep pricing pages; Indie Hackers; Stack Overflow Survey 2025; HN discussions*
