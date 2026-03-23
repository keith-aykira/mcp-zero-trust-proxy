---
phase: 05-marketing-seo-discoverability
plan: 03
subsystem: marketing
tags: [utm, seo, registry, outreach, sitemap, hackernews, reddit, discord]

# Dependency graph
requires:
  - phase: 05-01
    provides: FAQPage JSON-LD, sitemap with comparison page entries
  - phase: 05-02
    provides: 3 comparison pages (vs-sigbit, vs-kong, vs-cloudflare)
provides:
  - UTM-tracked Show HN post body (utm_source=hackernews)
  - UTM reference table with 8 channel-specific links in OUTREACH-MESSAGES.md
  - REGISTRY-LISTINGS.md with copy-paste submissions for 3 registries
  - Confirmed sitemap with all 5 page URLs
affects: [launch-day execution, traffic attribution, registry discoverability]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - UTM link pattern: utm_source={channel}&utm_medium={type}&utm_campaign=launch-2026&utm_content={subchannel}
    - Channel-specific UTM content param for Reddit (aiagents vs saas) and Discord (mcp-official vs claude-code)

key-files:
  created:
    - docs/go-to-market/REGISTRY-LISTINGS.md
  modified:
    - docs/go-to-market/SHOW-HN.md
    - docs/go-to-market/OUTREACH-MESSAGES.md

key-decisions:
  - "UTM link placeholders [USE UTM LINK FOR THIS CHANNEL] added to outreach templates — prevents Andrew from using wrong tracked link"
  - "awesome-mcp-servers PR is highest priority registry submission (most-starred MCP resource list, crawled by AI search engines)"
  - "Recommended registry submission order: awesome-mcp-servers -> Smithery -> Official MCP Registry (by traffic, review speed)"

patterns-established:
  - "UTM links always include utm_source, utm_medium, utm_campaign; channel variants add utm_content"
  - "Registry listing drafts include all required fields so submission is copy-paste with no research required on launch day"

requirements-completed: [MKT-01, MKT-06, MKT-07]

# Metrics
duration: 3min
completed: 2026-03-23
---

# Phase 5 Plan 03: Launch Package (UTM Tracking + Registry Listings) Summary

**UTM-tracked Show HN post, 8-channel outreach link table, and copy-paste registry listings for Official MCP Registry / Smithery / awesome-mcp-servers — all launch assets ready to execute**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-23T03:34:48Z
- **Completed:** 2026-03-23T03:38:11Z
- **Tasks:** 2
- **Files modified:** 3 (SHOW-HN.md, OUTREACH-MESSAGES.md, REGISTRY-LISTINGS.md new)

## Accomplishments

- Updated SHOW-HN.md: URL field and final body link now use utm_source=hackernews UTM link; test count updated from 212 to 236+; per-channel UTM reminder notes added to community seeding checklist
- Updated OUTREACH-MESSAGES.md: added UTM Link Reference table at top with 8 channel-specific URLs; added [USE UTM LINK FOR THIS CHANNEL] placeholder in every template body; added tip #7 reinforcing UTM link usage
- Created REGISTRY-LISTINGS.md: complete copy-paste listing content for all 3 target registries (Official MCP Registry, Smithery, awesome-mcp-servers PR body + exact README markdown line), submission checklist, and recommended ordering
- Confirmed sitemap.xml already contains all 5 URLs (home, 3 comparison pages, checkout-success) — no changes needed

## Task Commits

Each task was committed atomically:

1. **Task 1: Add UTM tracking to Show HN and outreach messages** - `17418d7` (feat)
2. **Task 2: Create registry listing drafts and finalize sitemap** - `fc63fb5` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `docs/go-to-market/SHOW-HN.md` — UTM links in URL field + post body, test count updated to 236+, per-channel UTM notes in seeding checklist
- `docs/go-to-market/OUTREACH-MESSAGES.md` — UTM Link Reference table with 8 channel links, [USE UTM LINK FOR THIS CHANNEL] placeholders in all template bodies
- `docs/go-to-market/REGISTRY-LISTINGS.md` — New file: copy-paste ready content for Official MCP Registry, Smithery, and awesome-mcp-servers PR; submission checklist and ordering rationale

## Decisions Made

- UTM placeholder text `[USE UTM LINK FOR THIS CHANNEL]` chosen over embedding the link inline — keeps the template generic and forces Andrew to consciously choose the right UTM per channel, reducing copy-paste errors
- awesome-mcp-servers PR flagged as highest leverage (most-starred list, likely crawled by AI search engines like Perplexity)
- Recommended submission order documented: awesome-mcp-servers first (passive organic) → Smithery (active marketplace) → Official MCP Registry (slower review cycle)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. All files are copy-paste-ready for launch day execution.

## Next Phase Readiness

Phase 5 is now complete. All three plans executed:
- Plan 05-01: FAQPage JSON-LD schema + sitemap with comparison page entries
- Plan 05-02: 3 comparison pages (vs-sigbit, vs-kong, vs-cloudflare)
- Plan 05-03: UTM tracking + registry listings (this plan)

**Launch day checklist:**
- Post Show HN (weekday 8–10am ET) using SHOW-HN.md
- Submit registry listings using REGISTRY-LISTINGS.md
- Seed communities using OUTREACH-MESSAGES.md (correct UTM link per channel)
- Still needed: swap Stripe test keys for live keys, tag v1.0.0

---
*Phase: 05-marketing-seo-discoverability*
*Completed: 2026-03-23*
