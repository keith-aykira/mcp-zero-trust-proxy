---
phase: 05-marketing-seo-discoverability
plan: 01
subsystem: seo
tags: [schema.org, json-ld, faq-schema, software-application, sitemap, rich-results, ai-search]

# Dependency graph
requires:
  - phase: 04-beta-launch-first-revenue
    provides: landing page (index.html) with initial SoftwareApplication JSON-LD block and sitemap.xml
provides:
  - FAQPage JSON-LD block with 5 Q&A pairs for Google rich results and AI citations
  - Enhanced SoftwareApplication JSON-LD with downloadUrl, softwareVersion, author
  - Expanded sitemap.xml with 5 URLs covering root, 3 comparison pages, checkout-success
affects:
  - 05-02 (comparison pages — sitemap entries now pre-declared)
  - 05-03 (AI discoverability — FAQ schema feeds LLM citation signals)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Dual JSON-LD blocks: SoftwareApplication (product metadata) + FAQPage (discoverability Q&A)"
    - "Sitemap pre-declares planned pages before they exist — crawlers discover on publish"

key-files:
  created: []
  modified:
    - landing-page/index.html
    - landing-page/sitemap.xml

key-decisions:
  - "FAQPage questions cover 5 buyer intents: setup, comparison, self-hosting, pricing, client compatibility"
  - "Comparison page sitemap entries added before pages exist — search engines crawl on first deploy"
  - "SoftwareApplication enhanced with downloadUrl pointing to GitHub Releases — supports direct download indexing"

patterns-established:
  - "Schema layering: SoftwareApplication for product entity + FAQPage for discoverability — both blocks coexist in <head>"
  - "Sitemap pre-population pattern: add planned page URLs before content is live"

requirements-completed: [MKT-02, MKT-03, MKT-04]

# Metrics
duration: 2min
completed: 2026-03-23
---

# Phase 05 Plan 01: FAQ Schema + Sitemap Expansion Summary

**FAQPage JSON-LD (5 Q&A pairs) + enhanced SoftwareApplication schema added to landing page; sitemap expanded to 5 URLs covering comparison pages and checkout**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-23T03:26:44Z
- **Completed:** 2026-03-23T03:27:43Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added FAQPage JSON-LD schema with 5 substantive Q&A pairs covering setup, competitor comparison, self-hosting, pricing, and client compatibility — signals needed for Google rich results and AI search citations
- Enhanced SoftwareApplication JSON-LD with `downloadUrl` (GitHub Releases), `softwareVersion` (1.0.0), and `author` fields — three previously missing fields that improve AI citation eligibility
- Expanded sitemap.xml from 1 to 5 URLs: added vs-sigbit, vs-kong, vs-cloudflare comparison pages and checkout-success — ensures crawlers index these as soon as pages go live

## Task Commits

Each task was committed atomically:

1. **Task 1: Add FAQ schema and enhance SoftwareApplication JSON-LD** - `1b53460` (feat)
2. **Task 2: Update sitemap.xml with planned page URLs** - `dfda699` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `landing-page/index.html` - Added second JSON-LD block (FAQPage) and enhanced existing SoftwareApplication block with 3 new fields
- `landing-page/sitemap.xml` - Expanded from 1 URL to 5 URLs with all planned comparison and transactional pages

## Decisions Made
- FAQPage questions selected from 5 highest-intent buyer questions: how to secure (setup), vs sigbit (comparison), self-hosted (data sovereignty), pricing, and client compatibility — these match exact search queries developers type
- Comparison page sitemap entries added before pages exist — search engines pre-discover URLs so crawl happens immediately on page publish
- downloadUrl points to GitHub Releases (not main repo) — more specific and signals binary availability to search engines

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- FAQ schema live in HTML — can be tested with Google Rich Results Test at https://search.google.com/test/rich-results
- Sitemap ready — submit to Google Search Console after next Vercel deploy
- Foundation in place for 05-02 (comparison page content) — sitemap entries already declared
- 05-03 (AI discoverability) can reference the 5 FAQ pairs as the core citation-bait content

---
*Phase: 05-marketing-seo-discoverability*
*Completed: 2026-03-23*
