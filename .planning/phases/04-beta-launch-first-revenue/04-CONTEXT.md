# Phase 4: Beta Launch & First Revenue - Context

**Gathered:** 2026-03-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Launch the hardened MCP Zero-Trust Proxy publicly, wire up Stripe billing with tiered pricing, deploy the landing page at mcpzerotrust.dev, publish Show HN, and convert early adopters to paid customers. Target: 10+ active deployments, $500+ MRR from 5+ paying customers.

</domain>

<decisions>
## Implementation Decisions

### Billing Integration
- License key model: proxy checks a signed JWT license key at startup
- Key contains tier, limits, and expiry — proxy enforces locally with zero network calls
- Free tier works without any license key (limited features, caps TBD by planner)
- Paid tiers require a valid license key obtained after Stripe checkout
- Key management service location: Claude's discretion (Supabase Edge Function is the likely fit given existing Supabase project)

### Launch Sequence
- Show HN is the FIRST move — go big immediately, no quiet beta period
- Landing page deploys to mcpzerotrust.dev same day as Show HN — everything goes live at once
- Breach data angle for Show HN: "8,000+ exposed MCP servers, 30 CVEs in 60 days" — matches existing template in `docs/go-to-market/OUTREACH-MESSAGES.md`
- Stripe billing live from day one — no free beta period, real revenue from launch
- Follow-up: community seeding (Discord, Reddit, X) AND direct DMs to interested developers in parallel
- Use existing outreach templates from `docs/go-to-market/OUTREACH-MESSAGES.md`

### Beta Onboarding
- No beta period — skip straight to public launch with paid tiers
- Self-serve Docker pull onboarding — must be frictionless
- QUICKSTART.md already exists from Phase 2 — build on it
- No white-glove onboarding — the product should be self-explanatory

### Tier Packaging
- Pricing: $49/$99/$199 per month (confirmed from PROJECT.md)
- $299 one-time binary license: Claude's discretion based on research (include if there's clear demand signal, defer if it complicates launch)
- Free tier limits: Claude's discretion — design limits that create natural upgrade pressure
- Tier enforcement: Claude's discretion — likely license key encodes tier + limits (aligns with license key decision above)
- Pricing page placement (on landing page vs separate): Claude's discretion

### Claude's Discretion
- License key management service architecture (Supabase Edge Function vs Go microservice)
- Free tier feature/limit boundaries (what creates natural upgrade pressure)
- Tier enforcement mechanism (JWT-encoded limits is the likely path)
- Launch timing specifics (exact day, HN posting time)
- "Active deployment" metric definition for BETA-01
- Whether to include $299 binary license at launch or defer
- Pricing page layout and placement
- Founding discount / launch promotion strategy

</decisions>

<specifics>
## Specific Ideas

- "It's got to be self-serve Docker pull, but we make it really easy" — frictionless onboarding is a core requirement
- Show HN angle already drafted in `docs/go-to-market/OUTREACH-MESSAGES.md` Version B — use and adapt it
- No beta testing period — the proxy has 177 tests with race detection, it's production-ready
- Same-day launch: landing page + Show HN + billing all go live together for maximum impact

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `docs/go-to-market/OUTREACH-MESSAGES.md`: Complete templates for Discord, DMs, Reddit, HN — ready to use
- `landing-page/index.html`: Built, wired to Supabase waitlist — needs Vercel deploy + pricing section
- `docs/QUICKSTART.md`: Docker setup guide — foundation for self-serve onboarding
- `docs/CONFIG-REFERENCE.md`: Full YAML reference — supports self-serve users
- Docker image: Working, 9.8MB binary, `CGO_ENABLED=0`

### Established Patterns
- YAML config with `${ENV_VAR}` substitution — license key can be injected via env var
- Config validation with `Validate()` — tier/license validation fits this pattern
- `internal/config/types.go`: All config types — license key config extends naturally here

### Integration Points
- `cmd/mcpproxy/main.go`: License key validation would run before server startup
- `internal/config/config.go`: License key parsing in `Load()` / `Validate()`
- Supabase project `dwumoznjyckebuirghne`: Waitlist table exists, can add license key table
- Stripe: No existing integration — new dependency

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-beta-launch-first-revenue*
*Context gathered: 2026-03-21*
