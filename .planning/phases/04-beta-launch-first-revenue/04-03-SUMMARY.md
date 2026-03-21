---
phase: 04-beta-launch-first-revenue
plan: "03"
subsystem: ui
tags: [landing-page, pricing, stripe, github-actions, ghcr, docker, show-hn, go-to-market]

# Dependency graph
requires:
  - phase: 04-01
    provides: JWT license key validation with tier enforcement in Go proxy
  - phase: 04-02
    provides: Stripe billing backend and Supabase Edge Functions for license generation

provides:
  - Landing page with 3-tier pricing section and Stripe checkout placeholders
  - GitHub Actions release workflow building Docker image for GHCR and multi-arch binaries
  - QUICKSTART.md updated with license key instructions for all tiers
  - Show HN submission draft with breach data angle and community seeding checklist

affects: [post-launch, outreach, revenue]

# Tech tracking
tech-stack:
  added: [github-actions, ghcr, docker/login-action, docker/build-push-action]
  patterns:
    - Tag-triggered GitHub Actions release workflow with test gate before publish
    - Multi-arch binary cross-compilation (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) via CGO_ENABLED=0
    - GHCR authentication via GITHUB_TOKEN (no PAT required)
    - STRIPE_PRO_LINK / STRIPE_ENTERPRISE_LINK placeholder pattern for user-replaceable URLs

key-files:
  created:
    - .github/workflows/release.yaml
    - docs/go-to-market/SHOW-HN.md
  modified:
    - landing-page/index.html
    - docs/QUICKSTART.md

key-decisions:
  - "Stripe checkout links use named placeholders (STRIPE_PRO_LINK / STRIPE_ENTERPRISE_LINK) so the agent can generate the page without live Stripe credentials — user replaces before deploy"
  - "GitHub Actions release workflow gates Docker push on passing tests — no broken images published"
  - "GHCR auth uses GITHUB_TOKEN only — no PAT required, zero manual secret setup"
  - "Show HN body leads with breach data (8,000+ exposed servers, 30+ CVEs) as the hook — evidence-first approach for technical audience"
  - "Community seeding checklist covers 7 channels with template references — user has exact steps for launch day"

patterns-established:
  - "Founding-member pricing badge above grid — lock-in framing for early adopters"
  - "Show HN structure: breach hook → problem → solution → one-liner demo → tech details → pricing → ask"

requirements-completed: [BETA-01, BETA-03, BETA-04]

# Metrics
duration: 20min
completed: 2026-03-21
---

# Phase 4 Plan 03: Beta Launch Package Summary

**3-tier pricing landing page, GHCR release workflow for Docker + multi-arch binaries, and Show HN submission ready to post — full launch package reviewed and approved.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-03-21
- **Completed:** 2026-03-21
- **Tasks:** 3 (Tasks 1-2 auto, Task 3 checkpoint approved by user)
- **Files modified:** 4

## Accomplishments

- Landing page pricing section added: Free ($0), Pro ($49/mo, recommended), Enterprise ($199/mo) with "Founding member pricing — locked in for life" badge and Stripe checkout placeholders
- GitHub Actions release workflow created: push tag `v*` triggers tests, then Docker push to GHCR and binary release for 4 platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) via GitHub Release
- QUICKSTART.md updated with license key instructions: free (no key), Pro/Enterprise (env var + config YAML), link to mcpzerotrust.dev
- Show HN draft written with breach-data hook, one-liner demo, technical details HN cares about (Go, 177 tests, race detection, ECDSA), and 7-channel community seeding checklist
- User reviewed and approved full launch package at Task 3 checkpoint

## Task Commits

Each task was committed atomically:

1. **Task 1: Landing page pricing + GHCR release workflow + QUICKSTART update** - `3d9ba84` (feat)
2. **Task 2: Draft Show HN post and community launch materials** - `b622e6b` (docs)
3. **Task 3: Review complete launch package before go-live** - checkpoint approved (no commit — human review task)

## Files Created/Modified

- `landing-page/index.html` — Added 3-tier pricing section with responsive grid, Stripe checkout placeholders, founding-member badge
- `.github/workflows/release.yaml` — Tag-triggered release: test gate → Docker/GHCR push → multi-arch binary GitHub Release
- `docs/QUICKSTART.md` — Added License Key section: tier descriptions, config YAML snippet, docker run with env var, link to mcpzerotrust.dev
- `docs/go-to-market/SHOW-HN.md` — Full Show HN submission with breach data angle, one-liner demo, pricing, community seeding checklist

## Decisions Made

- Stripe checkout links are placeholders (`STRIPE_PRO_LINK` / `STRIPE_ENTERPRISE_LINK`) so the landing page can be generated without live Stripe credentials — user replaces before deploying
- GHCR uses `GITHUB_TOKEN` only — no personal access token required, works out of the box
- Show HN leads with breach statistics as the hook (evidence-first for technical HN audience)
- "Founding member pricing — locked in for life" framing to create urgency for early adopters

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

Before going live, user must complete these steps (documented in plan's `<how-to-verify>` section):

1. Replace `STRIPE_PRO_LINK` and `STRIPE_ENTERPRISE_LINK` placeholders in `landing-page/index.html` with actual payment links from Plan 02's `setup-stripe-products.sh` output
2. Deploy landing page to Vercel: `vercel --prod` from `landing-page/` directory
3. Connect mcpzerotrust.dev domain in Vercel dashboard
4. Tag v1.0.0 and push to trigger GHCR release: `git tag v1.0.0 && git push origin v1.0.0`
5. Post Show HN from `docs/go-to-market/SHOW-HN.md` on a weekday 8-10am ET
6. Execute community seeding checklist (r/aiagents, r/SaaS, MCP Discord, Claude Code Discord, DM outreach, tweet thread)

## Next Phase Readiness

Phase 4 is fully complete from a code/content perspective. All deliverables are committed and approved:
- Landing page: ready to deploy (Stripe placeholders need replacement first)
- Docker image: will publish automatically on first `v*` tag push
- QUICKSTART.md: complete with license key instructions
- Show HN: ready to post

The remaining steps are manual user actions (deploy, tag, post). No further agent work required for Phase 4.

---
*Phase: 04-beta-launch-first-revenue*
*Completed: 2026-03-21*

## Self-Check: PASSED

Files verified:
- FOUND: .planning/phases/04-beta-launch-first-revenue/04-03-SUMMARY.md (this file)
- FOUND: 3d9ba84 — feat(04-03): landing page pricing + GHCR release workflow + QUICKSTART license docs
- FOUND: b622e6b — docs(04-03): draft Show HN post and community seeding checklist
