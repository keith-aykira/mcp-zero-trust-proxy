# MCP Zero-Trust Proxy — Current Handoff

**Phase:** 4 — Beta Launch & First Revenue (ALL 16 PLANS COMPLETE)
**Status:** All code and content committed and user-approved. Manual go-live steps remain.
**Last session:** 2026-03-21

## Recently Completed
- Plan 04-03: 3-tier pricing landing page, GHCR release workflow (Docker + 4-arch binaries on tag push), QUICKSTART license docs, Show HN draft — user-reviewed and approved
- Plan 04-02: Billing backend (ECDSA license keys + Stripe webhook Edge Functions + product setup script) — BETA-02/03 satisfied
- Plan 04-01: Go proxy binary with ES256 JWT license validation, free/pro/enterprise tier enforcement
- All 16 plans across 5 phases complete — 177 tests pass with -race

## Next Actions (Manual Go-Live Steps)

1. Replace Stripe placeholders in `landing-page/index.html`: `STRIPE_PRO_LINK` and `STRIPE_ENTERPRISE_LINK` with actual payment links from `scripts/setup-stripe-products.sh` output
2. Deploy landing page: `vercel --prod` from `landing-page/` directory; connect mcpzerotrust.dev in Vercel dashboard
3. Deploy Supabase Edge Functions: `supabase functions deploy create-license stripe-webhook`
4. Tag v1.0.0 and push: `git tag v1.0.0 && git push origin v1.0.0` — triggers GHCR Docker push + GitHub Release with multi-arch binaries automatically
5. Post Show HN on a weekday 8-10am ET — see `docs/go-to-market/SHOW-HN.md`
6. Execute community seeding checklist from SHOW-HN.md (r/aiagents, r/SaaS, MCP Discord, Claude Code Discord, DM outreach, tweet thread)

## Key References
- Launch package: `landing-page/index.html`, `.github/workflows/release.yaml`, `docs/QUICKSTART.md`, `docs/go-to-market/SHOW-HN.md`
- Billing backend: `supabase/functions/create-license/`, `supabase/functions/stripe-webhook/`, `scripts/`
- All summaries: `.planning/phases/04-beta-launch-first-revenue/04-0{1,2,3}-SUMMARY.md`
- GSD state: `.planning/STATE.md` (status: complete), `.planning/ROADMAP.md` (3/3 Phase 4 plans done)
- Notion project: `329532af-14ea-81fd-94a0-e88881c9d68e`
