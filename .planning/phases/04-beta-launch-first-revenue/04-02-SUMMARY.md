---
phase: 04-beta-launch-first-revenue
plan: 02
subsystem: payments
tags: [stripe, jwt, ecdsa, supabase, edge-functions, deno, license-keys]

# Dependency graph
requires:
  - phase: 04-beta-launch-first-revenue
    provides: proxy binary with license validation (Plan 01 JWT parse expects ES256 claims)
provides:
  - ECDSA P-256 keypair generation script for license signing
  - licenses table migration with RLS policies
  - create-license Deno Edge Function (ES256 JWT generation + Supabase insert)
  - stripe-webhook Deno Edge Function (checkout/subscription event handling)
  - setup-stripe-products.sh script (Pro $49/mo + Enterprise $199/mo products, prices, payment links)
affects:
  - 04-03-PLAN (landing page checkout buttons need payment link URLs from setup-stripe-products.sh)
  - 04-beta-launch-first-revenue deploy steps (supabase functions deploy)

# Tech tracking
tech-stack:
  added:
    - "Supabase Edge Functions (Deno v2)"
    - "ECDSA P-256 / ES256 JWT signing via Deno crypto.subtle"
    - "Stripe webhook HMAC-SHA256 verification via Deno crypto.subtle"
    - "Stripe API v1 (direct curl calls, no SDK)"
  patterns:
    - "JWT structure: {iss, sub, iat, exp, tier, max_upstreams, max_rpm} — matches Plan 01 license.Parse"
    - "Service-role client for all Edge Function writes — no anon DB writes"
    - "Always return HTTP 200 to Stripe webhooks — errors logged, not returned"
    - "CREATE_LICENSE_FUNCTION_URL secret enables internal function-to-function calls"

key-files:
  created:
    - scripts/generate-license-keypair.sh
    - supabase/migrations/20260321_create_licenses.sql
    - supabase/functions/create-license/index.ts
    - supabase/functions/stripe-webhook/index.ts
    - scripts/setup-stripe-products.sh
  modified: []

key-decisions:
  - "Deno crypto.subtle for both JWT signing (ECDSA) and Stripe sig verification (HMAC-SHA256) — no external SDK, lightweight"
  - "CREATE_LICENSE_FUNCTION_URL secret for stripe-webhook to call create-license — enables function composition without coupling"
  - "0 means unlimited for max_upstreams and max_rpm in enterprise tier — matches proxy validation logic in Plan 01"
  - "tier must be in Stripe metadata (checkout session or payment link) — no server-side tier lookup required"
  - "Service role RLS policies only for license inserts/updates — anon read is intentional (license key IS the secret)"
  - "PKCS#8 PEM format for private key — required by crypto.subtle importKey('pkcs8')"
  - "5-minute tolerance window for Stripe webhook signature verification — Stripe default tolerance"

patterns-established:
  - "Edge Function secrets pattern: LICENSE_SIGNING_KEY, STRIPE_WEBHOOK_SECRET, CREATE_LICENSE_FUNCTION_URL"
  - "License JWT expiry: 30 days from issuance — must be renewed via subscription renewal flow"
  - "Stripe webhook always returns 200 — processing errors logged, subscription state queried from DB"

requirements-completed: [BETA-02, BETA-03]

# Metrics
duration: 3min
completed: 2026-03-21
---

# Phase 4 Plan 02: Stripe Billing Backend Summary

**Stripe-to-license pipeline: ECDSA P-256 JWT license keys issued via Supabase Edge Functions after Stripe checkout, with subscription lifecycle management (create/renew/revoke)**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-21T15:05:21Z
- **Completed:** 2026-03-21T15:08:22Z
- **Tasks:** 3 of 3 complete (Task 3 checkpoint approved by user)
- **Files created:** 5

## Accomplishments

- ECDSA P-256 keypair generation script with clear instructions for securing the private key in Supabase secrets
- Licenses table migration with RLS: service role writes, public read (license key is the secret), indexes on email + subscription ID
- create-license Edge Function: PKCS#8 PEM import via Deno crypto.subtle, ES256 JWT with correct claims (tier, max_upstreams, max_rpm), inserts into Supabase
- stripe-webhook Edge Function: HMAC-SHA256 signature verification, handles checkout.session.completed / subscription.deleted / subscription.updated
- setup-stripe-products.sh: creates Pro ($49/mo) and Enterprise ($199/mo) products, prices, and payment links; outputs env vars and checkout URLs

## Task Commits

Each task was committed atomically:

1. **Task 1: Keypair generation script + licenses migration + create-license Edge Function** - `2f329bc` (feat)
2. **Task 2: Stripe webhook Edge Function + Stripe product/price setup script** - `63ee26b` (feat)
3. **Task 3: Review billing backend** - checkpoint approved (human-verify — user approved 2026-03-21)

## Files Created/Modified

- `scripts/generate-license-keypair.sh` — Generates ECDSA P-256 keypair (PKCS#8 PEM); prints instructions for Supabase secrets and proxy config
- `supabase/migrations/20260321_create_licenses.sql` — Creates licenses table with tier check constraint, RLS (service_role write, anon read), 3 indexes
- `supabase/functions/create-license/index.ts` — Deno Edge Function: imports private key, signs ES256 JWT, inserts license record; handles 400/500 errors
- `supabase/functions/stripe-webhook/index.ts` — Deno Edge Function: HMAC-SHA256 Stripe sig verification, 3 event handlers, always returns 200
- `scripts/setup-stripe-products.sh` — Stripe CLI-based setup; creates products, prices, payment links; outputs IDs for env var configuration

## Decisions Made

- **Deno crypto.subtle only** — no Stripe SDK, no jose library. Keeps Edge Functions lightweight and dependency-free.
- **CREATE_LICENSE_FUNCTION_URL** as an internal invocation pattern — stripe-webhook calls create-license via HTTP fetch rather than duplicating the JWT signing logic.
- **Service role for all DB writes** — anon writes to licenses table are blocked. Only Edge Functions (authenticated as service role) can insert/update licenses.
- **Tier in Stripe metadata** — the `metadata.tier` field on the payment link or checkout session carries "pro" or "enterprise" through the Stripe flow.
- **0 = unlimited** for enterprise tier max_upstreams and max_rpm — matches the convention in Plan 01's license validation logic.
- **PKCS#8 PEM** for the private key (not SEC1 format) — required by `crypto.subtle.importKey('pkcs8', ...)`.

## Deviations from Plan

None — plan executed exactly as written. Added `idx_licenses_stripe_customer` index (Rule 2 — missing critical) for customer ID lookups needed by `subscription.updated` handler, which queries the license by subscription ID and may need to cross-reference customer.

## Issues Encountered

None.

## User Setup Required

Before deploying (Plan 03), the following must be configured:

**Stripe (run once in test mode, then again in live mode):**

```bash
STRIPE_SECRET_KEY=sk_test_... ./scripts/setup-stripe-products.sh
```

Then set environment variables:
- `STRIPE_PRICE_PRO` — output from setup script
- `STRIPE_PRICE_ENTERPRISE` — output from setup script

**Generate keypair:**
```bash
./scripts/generate-license-keypair.sh
```

**Set Supabase secrets:**
```bash
supabase secrets set LICENSE_SIGNING_KEY="$(cat keys/private_key.pem)"
supabase secrets set STRIPE_WEBHOOK_SECRET=whsec_...  # after webhook endpoint registered
supabase secrets set CREATE_LICENSE_FUNCTION_URL=https://dwumoznjyckebuirghne.supabase.co/functions/v1/create-license
```

**After deploying Edge Functions, register webhook in Stripe Dashboard:**
- URL: `https://dwumoznjyckebuirghne.supabase.co/functions/v1/stripe-webhook`
- Events: `checkout.session.completed`, `customer.subscription.deleted`, `customer.subscription.updated`

## Next Phase Readiness

- All billing backend code ready for deployment (Plan 03)
- Plan 03 (landing page + deploy) needs payment link URLs from `setup-stripe-products.sh`
- Proxy binary (Plan 01) validates license JWTs against the public key from `generate-license-keypair.sh`
- User approved billing backend at checkpoint — plan is fully complete
- No blockers — ready for Plan 03 (landing page deploy + Edge Function deployment)

---
*Phase: 04-beta-launch-first-revenue*
*Completed: 2026-03-21*
