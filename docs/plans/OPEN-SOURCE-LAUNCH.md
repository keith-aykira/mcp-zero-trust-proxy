# Open-Source Launch Plan

**Date:** 2026-03-23
**Decision:** Pivot from closed-source SaaS to fully open-source free tool.
**Monetization:** Consulting lead-gen / reputation building.

---

## Decisions Made (from customer + YC panel review)

| # | Finding | Decision |
|---|---------|----------|
| 1 | Open-source | Fully open under MIT. No paid tiers. |
| 2 | Rename Enterprise tier | Remove all tiers. Single free product. |
| 3 | HN credibility | Let code speak. Don't volunteer bio. |
| 4 | Session Isolation claim | Rename → "Per-Client Sessions" everywhere |
| 5 | Sub-ms latency claim | Add p50/p95 specifics + benchmark script in repo |
| 6 | Race detector claim | Remove from landing page |
| 7 | Per-instance rate limiting | Document as known behavior (fits architecture) |
| 8 | Upstream timeout handling | Already handled (120s + 30s timeouts) ✓ |
| 9 | OAuth provider 500 | Already handled (clean errors) ✓ |
| 10 | 502/504 docs gap | Add to QUICKSTART troubleshooting section |
| 11 | SSE drop on reload | Document as known limitation |
| 12 | License local JWT | Confirmed local-only ✓ |
| 13 | Decompilation risk | Clean, no secrets ✓ |
| 14 | Free tier rate limits | Bump to 300 RPM / burst 100 (remove tier enforcement) |
| 15 | No Prometheus /metrics | Roadmap v1.1 |
| 16 | No Terraform/Helm | Roadmap (Helm ~2hrs when needed) |
| 17 | No SOC 2 | Roadmap |
| 18 | No SAML/SSO | Roadmap |
| 19 | Why closed-source response | N/A — now open-source |
| 20 | HN Q&A prep | Draft answers for 7 hardest questions |

---

## Chunk 1: Code Changes

- [ ] Remove tier enforcement from `cmd/mcpproxy/main.go` — all features available, no license key required
- [ ] Set default rate limits: 300 RPM, burst 100 (remove free/pro/enterprise distinction)
- [ ] Rename "Session Isolation" → "Per-Client Sessions" in code comments and docs
- [ ] Remove "Tested with Go's race detector" from any marketing-facing content
- [ ] Add benchmark script to repo (document p50/p95 methodology)
- [ ] Add 502/504 troubleshooting to QUICKSTART.md
- [ ] Document SSE reconnection behavior
- [ ] Document per-instance rate limiting behavior
- [ ] Verify all tests pass after changes

## Chunk 2: Landing Page Rewrite

- [ ] Remove pricing section (3-tier cards → single "Free & Open Source" message)
- [ ] Remove all Stripe payment links
- [ ] Remove license key references
- [ ] Change hero CTA → "View on GitHub" / "Get Started" (links to repo)
- [ ] Remove "Founding member pricing" badge
- [ ] Update feature comparison table (no tier columns)
- [ ] Rename "Session Isolation" → "Per-Client Sessions" in features
- [ ] Remove "race detector" from social proof bar
- [ ] Add "Open Source" badge/messaging prominently
- [ ] Keep: citations, FAQ, threat stats, "Stay in the loop" email capture
- [ ] Footer: keep support@mcpzerotrust.dev
- [ ] Update structured data (JSON-LD) — remove pricing, update description

## Chunk 3: Launch Prep

- [ ] Make GitHub repo public (currently private)
- [ ] Add MIT LICENSE file
- [ ] Add SECURITY.md (responsible disclosure policy)
- [ ] Update README.md for open-source audience
- [ ] Rewrite Show HN post — "Show HN: Open-source MCP security proxy"
- [ ] Draft answers for top 7 HN questions:
  1. "Why not Caddy + Authelia?"
  2. "How do rate limits work across replicas?"
  3. "What does per-client sessions actually mean?"
  4. "How do I monitor upstream failures?"
  5. "What's your bus factor?"
  6. "What does this do that nginx + OAuth2 Proxy doesn't?"
  7. "What happens when Anthropic adds native auth?"
- [ ] Deploy updated landing page to Vercel
- [ ] Tag v1.0.0 (triggers GitHub Release + Docker image)

---

## Positioning

**Before:** "Secure your MCP servers in minutes" (SaaS product)
**After:** "Open-source security proxy for MCP servers" (free tool, consulting lead-gen)

**One-liner for post-Anthropic world:** "Anthropic secures the connection. This proxy audits the data."

## What Goes Away

- Stripe integration (payment links, webhook, edge functions)
- License key system (JWT validation, tier enforcement)
- Pricing tiers (Free/Pro/Enterprise → just "free")
- checkout-success.html page
