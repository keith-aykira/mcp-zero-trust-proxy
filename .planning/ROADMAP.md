# Roadmap: MCP Zero-Trust Proxy

**Created:** 2026-03-19
**Core Value:** Any MCP server can be protected with enterprise-grade security in 5 minutes via a single Docker container — no code changes required.

## Phases

### Phase 0: Infrastructure (COMPLETE)
**Goal:** Git repo, Supabase waitlist, landing page wired and ready to deploy.
**Requirements:** INFRA-01, INFRA-02, INFRA-03
**Success Criteria:**
1. Git repo pushed to GitHub as private repo
2. Supabase project created with waitlist table and RLS policies
3. Landing page form submits emails to Supabase (verified with test insert)
4. Ready to deploy to Vercel when green-lit

### Phase 1: Deep Research (COMPLETE)
**Goal:** Build an evidence-based picture of market demand, competitive landscape, and technical feasibility before committing to building. This phase determines whether to proceed, pivot, or kill.
**Requirements:** RSCH-01, RSCH-02, RSCH-03, RSCH-04, RSCH-05, RSCH-06
**Plans:** 4 plans

Plans:
- [x] 01-01-PLAN.md — Pain signals + MCP ecosystem pulse (RSCH-01, RSCH-03)
- [x] 01-02-PLAN.md — Competitor movement + technical landscape (RSCH-02, RSCH-05)
- [x] 01-03-PLAN.md — Buyer behavior analysis (RSCH-04)
- [x] 01-04-PLAN.md — Go/no-go synthesis brief (RSCH-06)

**Success Criteria:**
1. 15+ real pain signals identified with direct quotes from developers (Reddit, Twitter/X, HN, Discord)
2. Updated competitive landscape showing current state of all known competitors + any new entrants
3. MCP adoption trajectory quantified (SDK downloads, server counts, enterprise signals)
4. Buyer behavior patterns documented with pricing benchmarks from comparable tools
5. Technical threat landscape updated (new CVEs, spec changes, platform auth announcements)
6. Go/no-go brief written scoring all 5 kill criteria with evidence from research

**Kill Criteria (evaluated at end of phase):**
- Pain is theoretical, not real (fewer than 5 genuine developer complaints found)
- A competitor has shipped a turnkey solution that closes our gap
- MCP spec or Anthropic/OpenAI is adding built-in auth that eliminates the need
- MCP adoption is stalling (flat or declining SDK downloads)
- No viable buyer segment willing to pay $49+/mo

**Research Lenses:**
1. **Pain Signals** — Reddit (r/aiagents, r/ClaudeAI, r/LocalLLaMA), Twitter/X, HN, Discord. Real developer complaints, workarounds, severity.
2. **Competitor Movement** — Updated status of 12 competitors from initial analysis. New entrants. GitHub activity (stars, commits, releases).
3. **MCP Ecosystem Pulse** — SDK download trends, new servers published, protocol spec changes, enterprise adoption signals.
4. **Buyer Behavior** — How teams buy API security tools. Pricing benchmarks. Purchase triggers. Solo dev vs team vs enterprise patterns.
5. **Technical Landscape** — New CVEs since Feb 2026, MCP spec auth updates, platform announcements, open-source implementations.

**Tools:** Exa deep research (pro), Exa advanced search (date-filtered, domain-filtered), Exa company research, Firecrawl for specific pages.

### Phase 2: MVP Build
**Goal:** Ship a functional, deployable proxy that protects real MCP servers.
**Requirements:** PRXY-01, PRXY-02, PRXY-03, PRXY-04, PRXY-05, PRXY-06, RBAC-01, RBAC-02, RBAC-03, DOCS-01, DOCS-02, DOCS-03
**Plans:** 6 plans

Plans:
- [ ] 02-01-PLAN.md — Go scaffolding, YAML config, core interfaces (PRXY-06)
- [ ] 02-02-PLAN.md — MCP reverse proxy core with JSON-RPC parsing and SSE (PRXY-02)
- [ ] 02-03-PLAN.md — OAuth 2.1 PKCE auth + session isolation (PRXY-01, PRXY-03)
- [ ] 02-04-PLAN.md — RBAC engine, audit logger, rate limiter (RBAC-01, RBAC-02, RBAC-03, PRXY-04)
- [ ] 02-05-PLAN.md — Pipeline wiring, main.go, Docker + binary packaging (PRXY-05)
- [ ] 02-06-PLAN.md — Integration tests (5 server types) + quick-start docs (DOCS-01, DOCS-02, DOCS-03)

**Success Criteria:**
1. `docker run` command protects any MCP server with OAuth 2.1 PKCE
2. RBAC enforces 3 built-in roles at the tool level
3. Audit log captures every MCP call with structured JSON
4. Session isolation prevents cross-tenant data leakage
5. Quick-start docs enable setup without help from the builder
6. Integration tests pass against 5 MCP server types

### Phase 3: Beta Launch & First Revenue
**Goal:** 10+ teams using proxy in production, convert 3-5 to paid.
**Requirements:** BETA-01, BETA-02, BETA-03, BETA-04
**Success Criteria:**
1. 10+ active proxy deployments in production
2. Stripe billing live with 3 pricing tiers
3. $500+ MRR from 5+ paying customers
4. Show HN post published with breach data angle

## Phase Summary

| # | Phase | Goal | Requirements | Success Criteria |
|---|-------|------|--------------|------------------|
| 0 | Infrastructure | Repo, Supabase, landing page ready | INFRA-01, INFRA-02, INFRA-03 | 4 (COMPLETE) |
| 1 | Deep Research | Evidence-based demand + feasibility picture | RSCH-01 through RSCH-06 | 6 (COMPLETE) |
| 2 | MVP Build | Ship deployable proxy | PRXY-01 through PRXY-06, RBAC-01 through RBAC-03, DOCS-01 through DOCS-03 | 6 |
| 3 | Beta Launch & Revenue | 10+ teams, first revenue | BETA-01 through BETA-04 | 4 |

---
*Roadmap created: 2026-03-19*
*Last updated: 2026-03-19 after Phase 2 planning (6 plans across 5 waves)*
