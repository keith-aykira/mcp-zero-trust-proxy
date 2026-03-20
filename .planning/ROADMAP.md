# Roadmap: MCP Zero-Trust Proxy

**Created:** 2026-03-19
**Core Value:** Any MCP server can be protected with enterprise-grade security in 5 minutes via a single Docker container — no code changes required.

## Phases

### Phase 0: Infrastructure
**Goal:** Git repo, Supabase waitlist, landing page wired and ready to deploy.
**Requirements:** INFRA-01, INFRA-02, INFRA-03
**Success Criteria:**
1. Git repo pushed to GitHub as private repo
2. Supabase project created with waitlist table and RLS policies
3. Landing page form submits emails to Supabase (verified with test insert)
4. Ready to deploy to Vercel when green-lit

### Phase 1: Validation Sprint (Days 1-7)
**Goal:** Prove real demand exists before writing product code.
**Requirements:** VALID-01, VALID-02, VALID-03, VALID-04, VALID-05
**Success Criteria:**
1. 15+ pain signals identified from scanning Shodan/Censys and community posts
2. 8+ substantive replies from DMs to developers running MCP servers
3. 2 of 3 discovery calls confirm weak/no auth AND willingness to pay
4. 25+ waitlist signups
5. Go/no-go brief written with evidence-based answers to 5 key questions

### Phase 2: MVP Build (Days 8-21)
**Goal:** Ship a functional, deployable proxy that protects real MCP servers.
**Requirements:** PRXY-01, PRXY-02, PRXY-03, PRXY-04, PRXY-05, PRXY-06, RBAC-01, RBAC-02, RBAC-03, DOCS-01, DOCS-02, DOCS-03
**Success Criteria:**
1. `docker run` command protects any MCP server with OAuth 2.1 PKCE
2. RBAC enforces 3 built-in roles at the tool level
3. Audit log captures every MCP call with structured JSON
4. Session isolation prevents cross-tenant data leakage
5. Quick-start docs enable setup without help from the builder
6. Integration tests pass against 5 MCP server types

### Phase 3: Beta Launch & First Revenue (Days 22-45)
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
| 0 | Infrastructure | Repo, Supabase, landing page ready | INFRA-01, INFRA-02, INFRA-03 | 4 |
| 1 | Validation Sprint | Prove demand before building | VALID-01 through VALID-05 | 5 |
| 2 | MVP Build | Ship deployable proxy | PRXY-01 through PRXY-06, RBAC-01 through RBAC-03, DOCS-01 through DOCS-03 | 6 |
| 3 | Beta Launch & Revenue | 10+ teams, first revenue | BETA-01 through BETA-04 | 4 |

---
*Roadmap created: 2026-03-19*
*Last updated: 2026-03-19 after initialization*
