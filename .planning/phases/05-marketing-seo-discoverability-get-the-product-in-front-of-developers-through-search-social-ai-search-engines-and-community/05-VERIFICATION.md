---
phase: 05-marketing-seo-discoverability
verified: 2026-03-22T21:00:00Z
status: passed
score: 10/10 must-haves verified
re_verification: false
human_verification:
  - test: "Open https://mcpzerotrust.dev in browser and run Google Rich Results Test"
    expected: "FAQPage schema validates with zero errors; 5 Q&A pairs appear"
    why_human: "Cannot hit live URL without deploying — Rich Results Test requires a crawlable URL"
  - test: "Navigate to /compare/vs-sigbit/, /compare/vs-kong/, /compare/vs-cloudflare/ on the deployed site"
    expected: "Pages render with correct styling (DM Serif Display headings, cream background), feature comparison tables, and CTA buttons that resolve to mcpzerotrust.dev"
    why_human: "Visual rendering and navigation flow require a browser"
---

# Phase 5: Marketing, SEO & Discoverability Verification Report

**Phase Goal:** Maximize search and AI discoverability through structured data, comparison pages, and UTM-tracked launch assets so developers evaluating MCP security can find, compare, and buy the proxy.
**Verified:** 2026-03-22
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | Landing page has valid FAQPage schema markup with 5+ questions | VERIFIED | `grep -c '"Question"' landing-page/index.html` returns 5; FAQPage block confirmed in second JSON-LD script tag |
| 2 | Landing page has enhanced SoftwareApplication schema with downloadUrl | VERIFIED | `downloadUrl`, `softwareVersion`, `author` all present in existing SoftwareApplication block |
| 3 | sitemap.xml has all planned comparison page entries | VERIFIED | 5 `<url>` entries confirmed: root, vs-sigbit, vs-kong, vs-cloudflare, checkout-success |
| 4 | Developer searching 'MCP auth proxy vs sigbit' finds a comparison page | VERIFIED | `landing-page/compare/vs-sigbit/index.html` exists, 463 lines, targets "mcp auth proxy alternative" meta |
| 5 | Developer searching 'Kong MCP alternative' finds a comparison page | VERIFIED | `landing-page/compare/vs-kong/index.html` exists, 459 lines, targets "kong mcp gateway alternative" meta |
| 6 | Comparison pages honestly acknowledge competitor strengths | VERIFIED | "Choose sigbit if...", "Choose Kong if...", "Choose Cloudflare if..." sections confirmed in each respective page |
| 7 | Each comparison page has OG tags, meta description, canonical, and JSON-LD | VERIFIED | All 3 pages have `og:title`, `og:description`, canonical, `meta name="description"`, and `application/ld+json` script tags |
| 8 | All comparison pages link back to main landing page | VERIFIED | `href.*mcpzerotrust.dev` confirmed in nav CTA and breadcrumb on all 3 pages |
| 9 | Show HN post body contains UTM-tracked link (utm_source=hackernews) | VERIFIED | 9 occurrences of `utm_source` in SHOW-HN.md; URL field and post body both use the tracked link; test count updated to "236+" |
| 10 | Outreach messages have UTM reference section and registry listing drafts exist | VERIFIED | "UTM Link Reference" table with 8 channel-specific URLs confirmed; REGISTRY-LISTINGS.md (133 lines) has copy-paste content for Official MCP Registry, Smithery, and awesome-mcp-servers PR |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `landing-page/index.html` | FAQPage JSON-LD + enhanced SoftwareApplication JSON-LD | VERIFIED | 2 JSON-LD blocks; 5 FAQ Questions; downloadUrl/softwareVersion/author present |
| `landing-page/sitemap.xml` | 5 URL entries with comparison pages | VERIFIED | Exactly 5 `<url>` entries; all 3 comparison page URLs present with lastmod/priority |
| `landing-page/compare/vs-sigbit/index.html` | Comparison page vs MCP Auth Proxy | VERIFIED | 463 lines; "vs MCP Auth Proxy" in content; OG tags; canonical; JSON-LD; CTA to main site |
| `landing-page/compare/vs-kong/index.html` | Comparison page vs Kong Gateway | VERIFIED | 459 lines; Kong content throughout; OG tags; canonical; JSON-LD; CTA |
| `landing-page/compare/vs-cloudflare/index.html` | Comparison page vs Cloudflare | VERIFIED | 460 lines; Cloudflare content throughout; OG tags; canonical; JSON-LD; CTA |
| `docs/go-to-market/SHOW-HN.md` | Show HN draft with UTM link (utm_source=hackernews) | VERIFIED | URL field and post body both use `utm_source=hackernews&utm_medium=show-hn&utm_campaign=launch-2026`; 236+ test count |
| `docs/go-to-market/OUTREACH-MESSAGES.md` | Outreach templates with UTM reference section | VERIFIED | "UTM Link Reference" table with 8 channel links; `[USE UTM LINK FOR THIS CHANNEL]` placeholders in templates |
| `docs/go-to-market/REGISTRY-LISTINGS.md` | Ready-to-submit registry listing drafts | VERIFIED | 133 lines; content for Official MCP Registry, Smithery, awesome-mcp-servers PR; submission checklist |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `landing-page/index.html` | schema.org | JSON-LD script tags (`application/ld+json`) | WIRED | 2 script blocks confirmed; `"@type": "FAQPage"` and `"@type": "SoftwareApplication"` both present |
| `landing-page/sitemap.xml` | comparison pages | URL `<loc>` entries | WIRED | `https://mcpzerotrust.dev/compare/vs-sigbit/`, `/vs-kong/`, `/vs-cloudflare/` all present |
| `landing-page/compare/vs-sigbit/index.html` | `https://mcpzerotrust.dev/` | CTA link (`href.*mcpzerotrust.dev`) | WIRED | Nav CTA + breadcrumb both resolve to mcpzerotrust.dev |
| `landing-page/compare/vs-kong/index.html` | `https://mcpzerotrust.dev/` | CTA link | WIRED | Nav CTA + breadcrumb both resolved |
| `landing-page/compare/vs-cloudflare/index.html` | `https://mcpzerotrust.dev/` | CTA link | WIRED | Nav CTA + breadcrumb both resolved |
| `docs/go-to-market/SHOW-HN.md` | `https://mcpzerotrust.dev` | UTM-tracked URL (`utm_source=hackernews`) | WIRED | 9 UTM occurrences; URL field uses tracked link |
| `docs/go-to-market/REGISTRY-LISTINGS.md` | `https://mcpzerotrust.dev` | Product URL in listing content | WIRED | `mcpzerotrust.dev` appears as listing URL in all 3 registry entries |

### Requirements Coverage

The plans claim MKT-01 through MKT-07 as requirement IDs. These IDs do NOT appear in `.planning/REQUIREMENTS.md` — the requirements file only tracks INFRA, RSCH, PRXY, RBAC, DOCS, BETA, and HARD namespaces. The MKT-* IDs are defined exclusively in the ROADMAP.md phase section and PLAN frontmatter.

This is a documentation gap (REQUIREMENTS.md was not extended for Phase 5), not an implementation gap. The ROADMAP.md success criteria serve as the requirement source of truth for this phase and all 6 are satisfied.

| Requirement | Source | Description | Status | Evidence |
|-------------|--------|-------------|--------|---------|
| MKT-01 | ROADMAP/05-03-PLAN | Show HN draft finalized with UTM link | SATISFIED | SHOW-HN.md has `utm_source=hackernews` in URL field and post body |
| MKT-02 | ROADMAP/05-01-PLAN | FAQ schema on landing page | SATISFIED | 5-question FAQPage JSON-LD block in index.html |
| MKT-03 | ROADMAP/05-01-PLAN | Enhanced SoftwareApplication schema | SATISFIED | downloadUrl, softwareVersion, author added to existing block |
| MKT-04 | ROADMAP/05-01-PLAN | sitemap.xml updated with all planned pages | SATISFIED | 5 URL entries confirmed |
| MKT-05 | ROADMAP/05-02-PLAN | 3 comparison pages at /compare/vs-*/ | SATISFIED | All 3 pages exist, are substantive (459-463 lines each), have OG/canonical/JSON-LD |
| MKT-06 | ROADMAP/05-03-PLAN | All outreach links have UTM tracking | SATISFIED | 8-channel UTM reference table + placeholders in every template |
| MKT-07 | ROADMAP/05-03-PLAN | Registry listing drafts for 3+ registries | SATISFIED | REGISTRY-LISTINGS.md has Official MCP Registry, Smithery, awesome-mcp-servers |

**Orphaned requirements note:** MKT-01 through MKT-07 do not exist in REQUIREMENTS.md. They exist only in ROADMAP.md and PLAN frontmatter. REQUIREMENTS.md should be updated to include the MKT-* namespace for completeness, but this does not block phase goal achievement.

### Anti-Patterns Found

No anti-patterns detected. Scans for TODO/FIXME/placeholder/stub patterns across all 8 modified files returned zero results. All comparison pages and go-to-market docs contain substantive, non-placeholder content.

### Human Verification Required

#### 1. Google Rich Results Test for FAQPage schema

**Test:** Navigate to https://search.google.com/test/rich-results, enter `https://mcpzerotrust.dev`, run test after next Vercel deploy.
**Expected:** FAQPage detected with 5 questions, zero errors or warnings. SoftwareApplication type also detected.
**Why human:** Requires the page to be deployed and crawlable — cannot verify with a local file grep.

#### 2. Visual rendering of comparison pages

**Test:** After Vercel deploy, open `/compare/vs-sigbit/`, `/compare/vs-kong/`, `/compare/vs-cloudflare/` in a browser.
**Expected:** Pages render with the cream/navy design system matching the main landing page; feature comparison tables are readable; CTA buttons are functional and navigate to `/#get-access`.
**Why human:** CSS rendering, visual consistency, and interactive element behavior cannot be verified by grep.

### Gaps Summary

No gaps. All 10 observable truths are verified. All 8 artifacts are present, substantive, and wired. All 7 requirement IDs from the plans are satisfied.

The only advisory item is that MKT-01 through MKT-07 are not recorded in REQUIREMENTS.md — this is a docs omission that does not affect the phase outcome.

---

_Verified: 2026-03-22_
_Verifier: Claude (gsd-verifier)_
