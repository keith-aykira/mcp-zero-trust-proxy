---
phase: 06-pre-launch-hardening
plan: "03"
subsystem: marketing
tags: [landing-page, copy, seo, faq, citations]

# Dependency graph
requires:
  - phase: 06-pre-launch-hardening
    provides: Expert review findings flagging inaccurate claims and missing FAQ content
provides:
  - Landing page with accurate test count (223), factual threat language, and 4 cited sources
  - FAQPage JSON-LD with platform-risk question (native auth objection)
  - Visible FAQ section with 3 entries answering developer concerns
  - Honest comparison table with "Choose nginx if..." guidance
  - OPEN-SOURCE-DECISION.md with full pros/cons and v1.0 decision
affects: [Show HN launch, HN community response, developer trust signals]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Threat stats linked to external citations (Shodan, CVE Details, NVD, Wiz)"
    - "Honest competitor framing with 'Choose X if...' notes"
    - "Visible FAQ section answering platform-risk and trust objections"

key-files:
  created:
    - docs/go-to-market/OPEN-SOURCE-DECISION.md
  modified:
    - landing-page/index.html
    - docs/go-to-market/SHOW-HN.md

key-decisions:
  - "Closed-source for v1.0 — competitive copying risk outweighs trust benefit before brand recognition established"
  - "Native auth objection addressed in both JSON-LD schema and visible FAQ rather than ignoring it"
  - "Honest 'Choose nginx/Kong if...' notes build more developer trust than superlative claims"

patterns-established:
  - "Claim accuracy: all test counts, performance stats must match actual verified output"
  - "Citation pattern: threat stats have linked sources in footnote style below each stat"

requirements-completed: [PLH-03, PLH-04, PLH-06, PLH-07, PLH-08, PLH-09]

# Metrics
duration: 12min
completed: 2026-03-23
---

# Phase 06 Plan 03: Landing Page Copy Hardening Summary

**Removed all HN-unsafe superlatives and unverified claims from landing page: corrected test count (212->223), cited all threat stats (Shodan/NVD/Wiz), answered the native auth objection in FAQ, dropped 'Race-condition free' and 'only drop-in proxy' language**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-03-23T22:00:00Z
- **Completed:** 2026-03-23T22:08:20Z
- **Tasks:** 3
- **Files modified:** 3 (index.html, SHOW-HN.md, + 1 created)

## Accomplishments

- Corrected test count to 223 and replaced vague "Race-condition free" with "Tested with Go's race detector" in social proof bar
- Changed "The MCP Security Crisis Is Now" to "The MCP threat landscape in 2026" and removed warning emoji from hero badge
- Added 4 linked source citations (Shodan, CVE Details, NVD, Wiz Research) below each threat stat
- Removed "only drop-in proxy" superlative; added honest "Choose nginx/Kong if..." guidance below comparison table
- Added "What if Anthropic adds native auth?" to FAQPage JSON-LD schema and as visible FAQ entry
- Created visible 3-entry FAQ section addressing platform risk, privacy, and open-source decision
- Created OPEN-SOURCE-DECISION.md documenting full pros/cons analysis with v1.0 closed-source decision

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix technical claims — test count and race detector language** - `6b57358` (fix)
2. **Task 2: Add threat stat citations and soften alarmist copy** - `0301350` (fix)
3. **Task 3: Add platform-risk FAQ and create open-source decision doc** - `9096f76` (feat)

## Files Created/Modified

- `landing-page/index.html` - Corrected test count, softened hero badge, updated threat heading, added 4 citations, removed superlative, added comparison guidance, added FAQ section, added FAQ JSON-LD entry
- `docs/go-to-market/SHOW-HN.md` - Updated test count from "236+" to "223 passing (go test -race)"
- `docs/go-to-market/OPEN-SOURCE-DECISION.md` - New file: pros/cons analysis and v1.0 decision to stay closed-source

## Decisions Made

- **Closed-source for v1.0:** Competitive copying risk is real before brand recognition. Trust concern addressed via citations, private security review offer, and honest FAQ copy instead.
- **Native auth FAQ is required:** The "what if Anthropic adds auth?" objection is universal on HN security tool posts — not answering it would be a red flag.
- **Factual language over marketing language:** "Tested with Go's race detector" is more credible to developers than "Race-condition free" which sounds like a marketing claim.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

The index.html file was being modified by a git pre-commit hook between edits, requiring fresh reads before each edit. No functional impact — all changes applied correctly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Landing page is now HN-ready: no unverifiable claims, all stats cited, platform-risk objection answered
- Phase 06 plan execution continues with remaining plans (04+)
- Show HN post and deployment can proceed after remaining phase 06 plans complete

---
*Phase: 06-pre-launch-hardening*
*Completed: 2026-03-23*
