---
phase: 05-marketing-seo-discoverability
plan: 02
subsystem: ui
tags: [html, seo, comparison-pages, og-tags, json-ld, landing-page]

requires:
  - phase: 05-marketing-seo-discoverability
    provides: "SEO foundation (OG tags, sitemap, robots.txt, llms.txt) from plan 05-01"

provides:
  - "Three bottom-of-funnel comparison pages targeting 'vs' keywords at /compare/vs-sigbit/, /compare/vs-kong/, /compare/vs-cloudflare/"
  - "Honest feature comparison tables for each competitor with 'choose each if' sections"
  - "OG tags, canonical URLs, JSON-LD breadcrumbs, and meta descriptions on each page"

affects: [05-03-marketing-seo, seo, landing-page, discoverability]

tech-stack:
  added: []
  patterns: [self-contained HTML pages with inlined CSS following main landing page design system, breadcrumb JSON-LD schema, comparison table pattern]

key-files:
  created:
    - landing-page/compare/vs-sigbit/index.html
    - landing-page/compare/vs-kong/index.html
    - landing-page/compare/vs-cloudflare/index.html
  modified: []

key-decisions:
  - "Honest competitor framing: each page acknowledges competitor strengths before advocating for MCP Zero-Trust Proxy"
  - "Inlined CSS only: no external stylesheet dependency, pages are fully self-contained"
  - "JSON-LD BreadcrumbList schema on each page for structured data"

patterns-established:
  - "Comparison page pattern: verdict box -> feature table -> choose-each cards -> about competitor -> CTA"
  - "Choose-each cards use accent-light background for 'ours' card and plain surface for competitor"

requirements-completed: [MKT-05]

duration: 4min
completed: 2026-03-23
---

# Phase 05 Plan 02: Competitor Comparison Pages Summary

**Three honest bottom-of-funnel comparison pages targeting high-intent 'vs' keywords: sigbit (auth-only), Kong (enterprise K8s), and Cloudflare (SaaS zero trust)**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-23T03:26:54Z
- **Completed:** 2026-03-23T03:30:54Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Created vs-sigbit comparison page targeting "mcp auth proxy alternative" and "sigbit vs" keywords, with honest acknowledgment that sigbit is the right choice for auth-only, zero-budget use cases
- Created vs-kong comparison page targeting "kong mcp gateway alternative" keywords, acknowledging Kong's maturity and plugin ecosystem while differentiating on MCP-specific features and setup simplicity
- Created vs-cloudflare comparison page targeting "cloudflare mcp alternative self-hosted" keywords, differentiating on self-hosting, tool-level RBAC, and data residency

## Task Commits

Each task was committed atomically:

1. **Task 1: Create vs-sigbit comparison page** - `ca7e653` (feat)
2. **Task 2: Create vs-kong and vs-cloudflare comparison pages** - `4ac2842` (feat)

## Files Created/Modified

- `landing-page/compare/vs-sigbit/index.html` - Comparison vs MCP Auth Proxy (sigbit): feature table, choose-each cards, OG tags, JSON-LD, CTA
- `landing-page/compare/vs-kong/index.html` - Comparison vs Kong Gateway: MCP-specific features vs general API management, Kubernetes overhead, pricing gap
- `landing-page/compare/vs-cloudflare/index.html` - Comparison vs Cloudflare Zero Trust: self-hosted vs SaaS, MCP JSON-RPC parsing, data residency, vendor lock-in

## Decisions Made

- Honest competitor framing was prioritized over pure advocacy: each page leads with a balanced verdict that may direct some users away from MCP Zero-Trust Proxy if it is not the right fit. This builds trust with developer audience that evaluates options skeptically.
- Each page is fully self-contained with inlined CSS (no external stylesheet), consistent with the plan requirement and avoids Vercel deploy complexity.
- The "Choose Kong if / Choose Kong Gateway if" heading was normalized to "Choose Kong if" to match plan verification check.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Pages are static HTML deployed as part of the landing-page Vercel deployment.

## Next Phase Readiness

- Three comparison pages live at /compare/vs-sigbit/, /compare/vs-kong/, /compare/vs-cloudflare/ (pending Vercel deploy)
- Pages are indexed via sitemap.xml from plan 05-01 (if sitemap was updated) or discoverable via Google crawl
- Ready for plan 05-03: FAQ schema, community seeding, or AI discoverability work

---
*Phase: 05-marketing-seo-discoverability*
*Completed: 2026-03-23*
