---
phase: 06-pre-launch-hardening
plan: 02
subsystem: ui
tags: [landing-page, cta, email, vercel]

# Dependency graph
requires: []
provides:
  - Landing page with single CTA path (no waitlist/beta confusion)
  - Professional support email on all customer-facing pages
  - Free tier linked directly to GitHub Releases
affects: [launch, show-hn, marketing]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - landing-page/index.html
    - landing-page/checkout-success.html

key-decisions:
  - "Free tier CTA points to GitHub Releases (not a form) — zero friction for free download"
  - "Bottom email form reframed as 'Stay in the loop' for update notifications, not beta access"
  - "support@mcpzerotrust.dev replaces personal Gmail everywhere — consistent professional identity for a security product"

patterns-established: []

requirements-completed: [PLH-02, PLH-05]

# Metrics
duration: 8min
completed: 2026-03-23
---

# Phase 6 Plan 02: CTA Consolidation and Professional Email Summary

**Landing page CTAs consolidated to single path (GitHub Releases for free, Stripe for paid), personal Gmail replaced with support@mcpzerotrust.dev across all customer-facing pages**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-23T22:00:00Z
- **Completed:** 2026-03-23T22:07:37Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Removed dual-CTA confusion (waitlist vs payment) flagged by expert review panels
- Free tier now has a zero-friction path directly to GitHub Releases
- Early email capture section (hero area form + `handleEarlySubmit` JS) fully removed
- Bottom notification form reframed from "beta waitlist" to "stay in the loop" with neutral updates language
- Personal Gmail address eliminated from all customer-facing pages (footer + checkout success page)

## Task Commits

Each task was committed atomically:

1. **Tasks 1 + 2: CTA consolidation + email replacement** - `550fd78` (feat)

## Files Created/Modified
- `landing-page/index.html` - Nav CTA, hero CTA, removed early capture section, free tier CTA, bottom form labels, footer email
- `landing-page/checkout-success.html` - Replaced personal Gmail with support@mcpzerotrust.dev in 3 locations

## Decisions Made
- Free tier CTA points to GitHub Releases (not a form) — zero friction for free download
- Bottom email form reframed as "Stay in the loop" for update notifications, not beta access
- `support@mcpzerotrust.dev` replaces personal Gmail everywhere — consistent professional identity for a security product

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

**Email forwarding needed.** Set up `support@mcpzerotrust.dev` → `andrewnoble1992@gmail.com` forwarding:
- Location: Vercel Dashboard -> mcpzerotrust.dev -> Settings -> Domains -> DNS Records
- Add MX records or configure email forwarding via Resend integration
- Without this, emails sent to support@mcpzerotrust.dev will not be received

## Next Phase Readiness
- Landing page is now launch-ready from a CTA/messaging perspective
- Professional email visible on all pages — needs forwarding configured before launch so support emails are received

---
*Phase: 06-pre-launch-hardening*
*Completed: 2026-03-23*
