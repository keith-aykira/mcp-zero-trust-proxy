---
phase: 04-beta-launch-first-revenue
plan: "01"
subsystem: license
tags: [jwt, ecdsa, license, billing, tier-enforcement, go-embed]

requires:
  - phase: 02-mvp-build
    provides: "Config system with env var substitution, main.go startup sequence, rate limit / audit config fields"
  - phase: 03-hardening
    provides: "Stable proxy pipeline with rate limiter wired, audit config stable"

provides:
  - "JWT license key parsing + validation using ECDSA P-256 (stdlib only)"
  - "FreeTierLicense() with 1 upstream / 10 RPM / stdout-only audit defaults"
  - "Pro tier: 5 upstreams, up to 200 RPM, file audit enabled"
  - "Enterprise tier: unlimited, all features"
  - "LicenseConfig in root Config struct with env var substitution support"
  - "ParseEmbedded() using bundled public key PEM via go:embed"
  - "Tier-based overrides applied at startup (rate limits, audit output)"

affects:
  - "04-02-billing-backend (Stripe checkout generates these JWTs)"
  - "docs — CONFIG-REFERENCE.md needs license section"

tech-stack:
  added:
    - "crypto/ecdsa (stdlib) — ECDSA P-256 signature verification"
    - "crypto/x509 + encoding/pem (stdlib) — public key PEM parsing"
    - "encoding/asn1 (stdlib) — DER-encoded ECDSA signature parsing"
    - "embed (stdlib) — bundled public key PEM in binary"
  patterns:
    - "TDD: RED (panic stubs) -> GREEN (full implementation) -> committed separately"
    - "External test package (license_test) — tests in _test.go use public API only"
    - "SignJWT() exported for tests only (documented); production keys generated offline"
    - "go:embed for public key — binary is self-contained, no runtime file deps"
    - "Tier enforcement via config override at startup (not middleware injection)"

key-files:
  created:
    - internal/license/license.go
    - internal/license/license_test.go
    - internal/license/license-signing-public.pem
    - keys/license-signing-public.pem
  modified:
    - internal/config/types.go
    - internal/config/config_test.go
    - cmd/mcpproxy/main.go
    - .gitignore

key-decisions:
  - "stdlib-only JWT: crypto/ecdsa + encoding/asn1 (no external JWT library) — reduces attack surface, no dependency management"
  - "ASN.1 DER signature encoding (not compact IEEE P1363) — matches openssl output format for offline key generation"
  - "ParseEmbedded() as the main.go entry point — binary is self-contained; no runtime key file required"
  - "Free tier enforced via config override at startup — avoids adding license checks throughout middleware chain"
  - "Private key never committed — .gitignore excludes keys/license-signing-private.pem; public key only in repo"
  - "Zero network calls for validation — all claims are in the signed JWT; air-gapped deployments work"

patterns-established:
  - "License key format: ES256 JWT with DER-encoded ECDSA P-256 signature, base64url segments"
  - "Tier enforcement: override cfg fields before component init, not via middleware"

requirements-completed: [BETA-01, BETA-02]

duration: 6min
completed: "2026-03-21"
---

# Phase 4 Plan 01: JWT License Key Validation Summary

**ECDSA P-256 signed JWT license keys with local tier enforcement — free/pro/enterprise tiers embedded in proxy binary via go:embed, zero network calls required**

## Performance

- **Duration:** 6 minutes
- **Started:** 2026-03-21T15:05:38Z
- **Completed:** 2026-03-21T15:11:47Z
- **Tasks:** 4 (RED, GREEN, config types, main.go wiring)
- **Files modified:** 7

## Accomplishments

- Built complete JWT license validation using only Go stdlib (crypto/ecdsa, encoding/asn1, crypto/x509) — no external JWT library
- 11 license tests + 3 config tests pass with -race flag; zero regressions in the 177+ existing tests (203 total now pass)
- Tier enforcement applied at startup via config field overrides: free tier caps rate at 10 RPM + forces stdout audit; pro tier caps at MaxRPM from JWT; enterprise tier uncapped

## Task Commits

Each task was committed atomically:

1. **Task RED: Failing license tests** - `2552b3a` (test)
2. **Task GREEN: License package implementation** - `aaa5194` (feat)
3. **Task: LicenseConfig in config types** - `ff79fa8` (feat)
4. **Task: Wire into main.go startup** - `fb27072` (feat)

_TDD plan: RED and GREEN committed separately as required_

## Files Created/Modified

- `internal/license/license.go` — License, Tier types; Parse(), Validate(), FreeTierLicense(), ParseEmbedded(), SignJWT(), PublicKeyFromPEM()
- `internal/license/license_test.go` — 11 tests: free tier, valid pro/enterprise JWT, expired, tampered, wrong issuer, wrong key, malformed
- `internal/license/license-signing-public.pem` — ECDSA P-256 public key embedded via go:embed
- `keys/license-signing-public.pem` — Same public key in project root keys/ directory
- `internal/config/types.go` — LicenseConfig struct + License field added to root Config
- `internal/config/config_test.go` — 3 new tests: explicit key, empty key (free tier), env var substitution
- `cmd/mcpproxy/main.go` — license.ParseEmbedded() after config.Validate, tier overrides applied, startup log
- `.gitignore` — Added keys/license-signing-private.pem exclusion

## Decisions Made

- **stdlib-only JWT**: Using `crypto/ecdsa` + `encoding/asn1` + `crypto/x509` — no `golang-jwt/jwt` or `lestrrat-go/jwx` dependency. Reduces attack surface and avoids JWT library CVEs.
- **ASN.1 DER signature**: Matches `openssl dgst -sign` output format, making offline key generation straightforward with standard tools.
- **ParseEmbedded() as the main.go API**: Binary is self-contained — no runtime key file paths to manage or secure in production deployments.
- **Config override at startup for tier enforcement**: Rather than adding license checks to every middleware handler, tier limits override the config fields before components are initialized. Simpler and more auditable.
- **Private key never committed**: `.gitignore` explicitly excludes `keys/license-signing-private.pem`. The private key stays on the operator's machine for signing customer licenses.
- **Zero network calls**: All validation is local — JWT carries everything. Air-gapped and offline deployments work without modification.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

The license signing private key was generated at `/Users/andrewnoble/Developer/mcp-zero-trust-proxy/keys/license-signing-private.pem`. This key is excluded from git and must be stored securely.

To generate a customer license JWT:
```bash
# Use the generate-license.sh script (to be created in 04-02 billing plan)
# or manually with openssl + base64url encoding
```

The public key embedded in the binary is at `internal/license/license-signing-public.pem`. If you rotate the signing key, replace this file and rebuild.

## Next Phase Readiness

- License validation infrastructure is complete and tested
- 04-02 (Billing Backend) needs to generate these JWTs when a customer pays via Stripe
- CONFIG-REFERENCE.md needs a `license:` section documenting the key field

---
*Phase: 04-beta-launch-first-revenue*
*Completed: 2026-03-21*
