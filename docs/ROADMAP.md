# MCP Zero-Trust Proxy — 90-Day Roadmap

**Owner:** Andrew Noble
**Start date:** Week of March 23, 2026
**Product:** Drop-in authentication middleware proxy for MCP servers
**Composite score:** 8.1 / 10 (ranked #1 of 49 ideas across 10 research reports)
**Status:** Pre-validation

---

## Executive Summary

MCP Zero-Trust Proxy is a lightweight reverse-proxy that sits in front of any MCP server and enforces OAuth 2.1 PKCE authentication, token exchange, cryptographic validation, session isolation, and role-based access control. It addresses an acute, time-bounded security gap: the Clawdbot breach (January 2026) exposed thousands of MCP servers, and independent scans found 8,000+ unauthenticated MCP endpoints in February 2026. No turnkey proxy solution exists today — only scanners (Proximity, CyberMCP, MCPSafetyScanner) that detect problems but don't fix them.

This roadmap covers 90 days from first customer contact to 100+ active proxies and 10+ paying teams — the 3-month validation check from the deep dive analysis.

---

## Key Numbers

- **TAM:** $1.8B (MCP/API security market)
- **SOM:** 1,000–2,000 teams (Year 1 addressable)
- **Pricing:** $49/mo Starter · $99/mo Pro · $199/mo Enterprise (or $299 binary license)
- **Revenue ceiling:** $15–30K/mo at scale
- **Conservative Y1 cumulative:** $19,478
- **Aggressive Y1 cumulative:** $68,008
- **MVP build time:** 10–14 days
- **Months to $5K MRR:** 7 (aggressive) / 11 (conservative)
- **Competitors:** Proximity, CyberMCP, MCPSafetyScanner (OSS scanners only — no active proxy), Kong/Salt Security (enterprise, 3–5 month lag)
- **Trigger date:** January 2026 (Clawdbot breach + 8K exposed servers)

---

## Phase 1: Validation Sprint (Days 1–7)

**Goal:** Prove real demand exists before writing a line of product code.

### Day 1 (Mon) — Map the Attack Surface
- Scan Shodan/Censys for exposed MCP servers
- Catalog Clawdbot breach details and timeline
- Search MCP Discord, r/aiagents, HN for security complaints and workaround posts
- Build a list of 20 developers who have posted about MCP security concerns
- **Done signal:** 15+ pain signals identified, 10+ DM-ready devs, 5+ exposed server examples
- **Kill signal:** Fewer than 3 relevant posts in 30 days — pain is theoretical, not real

### Day 2 (Tue) — Direct Outreach
- DM 15 devs/teams running MCP servers
- Post in MCP Discord: "How do you secure your MCP endpoints today?"
- Join Claude Code community channels and observe discussion
- **Done signal:** 8+ substantive replies describing security concerns and current workaround (or lack thereof)
- **Kill signal:** Zero replies, or consistent response of "auth isn't a problem for us"

### Day 3 (Wed) — Discovery Calls
- Schedule and run 3 calls with the most engaged respondents
- Discovery questions: How many MCP servers do you run? What auth method (if any)? Worst security scare? What would you pay for a drop-in proxy? What's your deployment environment?
- **Done signal:** 2 of 3 describe zero or weak auth AND willing to pay $49+/mo
- **Kill signal:** All 3 already solved it with nginx/Caddy reverse proxy configs, or don't care

### Day 4 (Thu) — Prototype + Competitive Audit
- Build minimal MCP auth proxy: OAuth 2.1 PKCE flow + token validation layer
- Test against 3 popular MCP server types (Claude Code, custom, community)
- Deep-dive evaluate Proximity, CyberMCP, and any Kong MCP support
- **Done signal:** Proxy intercepts and validates MCP calls end-to-end. Clear feature gaps in existing tools.
- **Kill signal:** Existing tools already solve 80%+ of the problem for free

### Day 5 (Fri) — Landing Page + Pricing Test
- Ship landing page: "Secure your MCP servers in 5 minutes"
- 3 pricing tiers displayed, email capture form
- Share in communities contacted on Tuesday
- **Done signal:** 25+ email signups over the weekend
- **Kill signal:** Fewer than 5 signups — value proposition not compelling enough

### Day 6 (Sat) — Synthesize + Go/No-Go Decision
- Review all evidence from the week
- Write a 1-page go/no-go brief scoring 5 key questions:
  1. Is the pain real and urgent? (evidence from Day 1–2)
  2. Will people pay? (evidence from Day 3)
  3. Can we build a differentiated product? (evidence from Day 4)
  4. Is there a reachable audience? (evidence from Day 5)
  5. Is the timing right? (competitive window assessment)
- **Done signal:** All 5 questions answered with evidence. Clear demand.
- **Kill signal:** 2+ questions answered negatively

### Day 7 (Sun) — Commit or Pivot
- **If GO:** Begin full proxy build, write docs skeleton, plan Week 2 sprint
- **If NO-GO:** Pivot to AI Cost Attribution validation sprint starting Monday

---

## Phase 2: MVP Build (Days 8–21)

**Goal:** Ship a functional, deployable proxy that protects real MCP servers.

### Week 2 (Days 8–14) — Core Proxy Engine

- **Auth layer:** OAuth 2.1 PKCE flow with token exchange and refresh
- **Proxy engine:** Transparent reverse proxy (no MCP server code changes required)
- **Session isolation:** Per-client session boundaries preventing cross-tenant data leakage
- **Logging:** Structured audit log of every MCP call (who, what, when, allowed/denied)
- **Deployment:** Single Docker container or single binary (both options)
- **Config:** YAML-based configuration — server URL, allowed clients, token issuer
- **Milestone:** Proxy running in front of a real MCP server, blocking unauthenticated requests, passing authenticated ones cleanly

### Week 3 (Days 15–21) — RBAC + Dashboard + Docs

- **RBAC:** Role-based policies (admin, read-only, tool-restricted) per client
- **Dashboard:** Minimal web UI showing active sessions, blocked requests, audit trail
- **Rate limiting:** Per-client request throttling to prevent abuse
- **Documentation:** Quick-start guide, Docker/binary install, configuration reference
- **Integration tests:** Automated test suite against 5 MCP server types
- **Milestone:** Feature-complete MVP ready for beta testers from Week 1 outreach

### MVP Definition of Done
- [ ] Docker pull + single `docker run` command protects any MCP server
- [ ] OAuth 2.1 PKCE authentication working end-to-end
- [ ] RBAC with at least 3 built-in roles
- [ ] Audit log queryable from dashboard
- [ ] Rate limiting configurable per client
- [ ] Quick-start docs written and tested by someone who didn't build it
- [ ] Zero known security vulnerabilities in proxy itself

---

## Phase 3: Beta Launch + First Revenue (Days 22–45)

**Goal:** Get 10+ teams using the proxy in production, convert 3–5 to paid.

### Week 4 (Days 22–28) — Beta Distribution

- Deploy beta to 10–15 teams from Week 1 waitlist and discovery calls
- Set up feedback channel (Discord channel or GitHub Discussions)
- Instrument usage analytics: active proxies, requests proxied, auth failures blocked
- Write and publish a technical blog post: "We scanned 8,000 exposed MCP servers. Here's what we found."
- **Milestone:** 10+ active proxy deployments in production environments

### Week 5 (Days 29–35) — Iterate on Feedback

- Bug fixes and stability improvements based on beta feedback
- Add top 3 requested features (prioritize by frequency)
- Harden for edge cases: WebSocket MCP transport, streaming responses, large payloads
- Begin Stripe integration for billing
- **Milestone:** Zero critical bugs, beta NPS > 40

### Week 6 (Days 36–42) — Public Launch

- **Hacker News "Show HN" post** — lead with the breach data and before/after security comparison
- Post in MCP Discord, r/aiagents, r/SaaS, Claude Code community
- Activate paid tiers: $49/mo Starter (1 server), $99/mo Pro (5 servers), $199/mo Enterprise (unlimited + SSO)
- **Email sequence** to waitlist: free trial → paid conversion
- **Milestone:** 25+ email signups convert to free trial, 3–5 paid subscribers

### Weeks 6–7 (Days 42–49) — First Revenue Push

- Direct outreach to teams running 5+ MCP servers (pro/enterprise targets)
- Offer "founding customer" annual pricing (20% discount for annual commit)
- Write case study from most active beta user
- **Milestone:** $500+ MRR, 5+ paying customers

---

## Phase 4: Growth + Validation (Days 46–90)

**Goal:** Reach 100+ active proxies, 10+ paying teams, and validate product-market fit.

### Weeks 7–9 (Days 46–63) — Distribution Engine

- **Content marketing:** Publish weekly security findings from anonymized proxy data ("This week in MCP security")
- **Community presence:** Become the go-to MCP security voice in Discord/forums
- **Partnerships:** Reach out to MCP server framework maintainers for "recommended security" mention in docs
- **SEO:** Target "MCP server security," "MCP authentication," "secure MCP deployment"
- **Milestone:** 50+ active proxies, organic inbound leads from content

### Weeks 9–11 (Days 63–77) — Product Expansion

- **Team features:** Shared policies across multiple proxies, centralized dashboard
- **Alerting:** Real-time notifications for auth failures, suspicious patterns, new clients
- **Compliance:** SOC 2-ready audit log export, data residency options
- **API:** Programmatic proxy management for teams with 10+ servers
- **Milestone:** Enterprise pilot conversations started with 2+ mid-market teams

### Weeks 11–13 (Days 77–90) — PMF Checkpoint

- **Quantitative check:**
  - [ ] 100+ active proxies deployed
  - [ ] 10+ paying teams
  - [ ] $2,000+ MRR
  - [ ] Monthly churn < 10%
  - [ ] At least 1 documented security incident caught by proxy
- **Qualitative check:**
  - [ ] 3+ unsolicited referrals (word-of-mouth)
  - [ ] Teams upgrading from Starter to Pro/Enterprise
  - [ ] Feature requests focused on "more" (more servers, more policies) not "fix" (bugs, instability)
- **Decision:** If 4 of 5 quantitative checks pass → double down, hire first contractor, plan Phase 5
- **Decision:** If 2+ quantitative checks fail → analyze why, consider pivot to adjacent product or different buyer segment

---

## Kill Criteria (Check Weekly)

These are the signals that should trigger a pause-and-reassess at any point during the 90 days:

1. **No pain signal:** Fewer than 5 developers express real security concern by end of Week 1
2. **Won't pay:** Fewer than 3 people willing to pay $49+/mo after seeing the product by end of Week 4
3. **Competitor closes gap:** Kong, Salt Security, or Cloudflare ships a turnkey MCP auth proxy
4. **Technical blocker:** MCP protocol changes make proxy approach infeasible
5. **Market timing:** MCP adoption stalls or major platform (Anthropic, OpenAI) ships built-in auth that eliminates the need

---

## Revenue Projections (Months 1–12)

### Conservative (15% monthly growth)

| Month | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | Y1 Total |
|-------|---|---|---|---|---|---|---|---|---|----|----|-----|----------|
| MRR   | $0 | $800 | $920 | $1,058 | $1,217 | $1,399 | $1,609 | $1,850 | $2,128 | $2,447 | $2,814 | $3,236 | $19,478 |

### Aggressive (25% monthly growth, 2x M1)

| Month | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | Y1 Total |
|-------|---|---|---|---|---|---|---|---|---|----|----|-----|----------|
| MRR   | $0 | $1,600 | $2,000 | $2,500 | $3,125 | $3,906 | $4,883 | $6,104 | $7,630 | $9,537 | $11,921 | $14,902 | $68,008 |

Crosses $5K MRR by Month 7 (aggressive) or Month 11 (conservative) — fastest of all 49 ideas.

---

## Tech Stack (Recommended)

- **Proxy core:** Go or Rust (single-binary deployment, low latency, small footprint)
- **Auth:** OAuth 2.1 PKCE via built-in library (no external IdP dependency for Starter tier)
- **Dashboard:** Next.js + Tailwind (you already know this stack)
- **Database:** SQLite for single-node, Supabase/Postgres for cloud dashboard
- **Billing:** Stripe (existing integration experience)
- **Deployment:** Docker image + standalone binary + optional Helm chart
- **CI/CD:** GitHub Actions for build, test, release
- **Monitoring:** Built-in /health endpoint + optional Prometheus metrics export

---

## Parallel Plays (If Validation Succeeds)

The executive report recommends launching parallel products once MCP Zero-Trust Proxy is validated. These share the same buyer (AI-native developers) and technical infrastructure:

| Product | Launch Window | Trigger to Start | Relationship to Proxy |
|---------|--------------|-------------------|----------------------|
| **Safer Auto Mode** (R10, score 7.5) | Weeks 3–5 | Proxy MVP shipped + 5 beta users | Trust layer for agentic coding — adjacent security product, same buyer |
| **AI Cost Attribution** (R7, score 8.0) | Weeks 6–10 | Proxy has 10+ paying teams | Billing layer — different problem, same infrastructure (Stripe + Supabase) |
| **Plugin & Skill Ops** (R10, score 7.3) | Weeks 8–12 | Safer Auto Mode validated | CI/CD for Claude plugins — extends trust/security thesis into dev tooling |

The decision to start a parallel play should only happen after the primary product is past its Week 4 beta milestone. Don't split focus earlier.

---

## Fallback Plan

If MCP Zero-Trust Proxy fails validation (Week 1 kill criteria hit), pivot immediately to:

1. **AI Cost Attribution** (score 8.0) — 4–6 week MVP, Stripe trigger event, $200M SAM
2. **StockPilot** (score 7.7) — Shopify forced migration, marketplace distribution, lower technical risk
3. **FBA Prep Compliance** (score 7.7) — Amazon enforcement deadline, marketplace-native buyers

---

*Generated from synthesis of 10 research reports, 49 scored opportunities, and deep-dive analysis. Last updated March 19, 2026.*
