# Launch Readiness Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all launch blockers identified by C-suite review so the product can accept real payments, deliver license keys to customers, and present honest claims on the landing page.

**Architecture:** 6 tasks with dependencies. Task 1 (copy fixes), Task 4 (support email), and Task 5 (Show HN) modify the landing page and should be serialized. Task 2 (checkout success page + license delivery) must deploy before Task 3 (production Stripe) can be tested end-to-end. Task 6 (Show HN prep) is last since it references everything else.

**Execution order:** Task 1 → Task 4 (both touch index.html) → Task 2 (checkout page + license delivery) → deploy → Task 3 (production Stripe, depends on checkout page existing) → Task 5 (Show HN, last)

**Tech Stack:** HTML/CSS/JS (landing page), Stripe API, Vercel, Supabase, Deno (Edge Functions)

---

## Task 1: Fix Overstated Claims on Landing Page

**Files:**
- Modify: `landing-page/index.html`

This task fixes every claim-vs-reality gap identified in the review.

- [ ] **Step 1: Fix "5 minutes" claim**

In `landing-page/index.html`, make these changes:

Line 6 — title tag:
```html
<!-- OLD -->
<title>MCP Zero-Trust Proxy — Secure Your MCP Servers in 5 Minutes</title>
<!-- NEW -->
<title>MCP Zero-Trust Proxy — Secure Your MCP Servers in Minutes</title>
```

Line 364 — hero h1:
```html
<!-- OLD -->
<h1>Secure your MCP servers <em>in 5 minutes</em></h1>
<!-- NEW -->
<h1>Secure your MCP servers <em>in minutes</em></h1>
```

Line 514 — comparison table "5 min" cell:
```html
<!-- OLD -->
<td class="us-col" style="color: var(--green); font-weight: 700;">5 min</td>
<!-- NEW -->
<td class="us-col" style="color: var(--green); font-weight: 700;">&lt; 10 min</td>
```

- [ ] **Step 2: Fix audit log claims**

Line 427 — feature card description:
```html
<!-- OLD -->
<p>Every MCP call logged: who, what, when, allowed or denied. SOC 2-ready export. SIEM integration via OpenTelemetry.</p>
<!-- NEW -->
<p>Every MCP call logged: who, what, when, allowed or denied. Structured JSON export. File rotation by size and age.</p>
```

- [ ] **Step 3: Fix Enterprise tier feature list**

Lines 573-574 — Enterprise pricing card:
```html
<!-- OLD -->
<li>SOC 2-ready audit export</li>
<li>SIEM integration (OTel)</li>
<!-- NEW -->
<li>Structured audit log export (JSON)</li>
<li>SIEM integration (coming soon)</li>
```

- [ ] **Step 4: Fix social proof section**

Lines 586-597 — proof section:
```html
<!-- OLD -->
<div class="proof-logos">
  <span>OWASP</span>
  <span>•</span>
  <span>MICROSOFT</span>
  <span>•</span>
  <span>PALO ALTO UNIT 42</span>
  <span>•</span>
  <span>ANTHROPIC</span>
</div>
<p>Leading security organizations recommend MCP server authentication.<br>We make it effortless to implement.</p>
<!-- NEW -->
<div class="proof-logos">
  <span>OWASP</span>
  <span>•</span>
  <span>MICROSOFT</span>
  <span>•</span>
  <span>PALO ALTO UNIT 42</span>
  <span>•</span>
  <span>ANTHROPIC</span>
</div>
<p>These organizations have published guidance on securing MCP servers.<br>We built the tool that makes it easy.</p>
```

- [ ] **Step 5: Fix "5-Minute Setup" feature card title**

Line 431 — feature card:
```html
<!-- OLD -->
<h3>5-Minute Setup</h3>
<p>Docker pull, set your MCP target and auth provider, done. Single container, single config file. No Kubernetes required.</p>
<!-- NEW -->
<h3>Quick Setup</h3>
<p>Docker pull, set your MCP target and auth provider, done. Single container, single config file. No Kubernetes required.</p>
```

- [ ] **Step 6: Verify all changes render correctly**

Open `landing-page/index.html` in a browser. Check:
- Hero says "in minutes" (not "in 5 minutes")
- Audit feature card says "Structured JSON export"
- Enterprise tier says "coming soon" for SIEM
- Social proof says "published guidance" not "recommend"
- Comparison table says "< 10 min"

- [ ] **Step 7: Commit**

```bash
git add landing-page/index.html
git commit -m "fix(landing): correct overstated claims — timing, audit, social proof"
```

---

## Task 2: Create Checkout Success Page + License Key Retrieval

**Files:**
- Create: `landing-page/checkout-success.html`
- Create: `supabase/functions/get-license/index.ts`

This is the most critical missing piece. Stripe redirects to `https://mcpzerotrust.dev/checkout-success?tier=pro&session_id={CHECKOUT_SESSION_ID}` after payment. That page doesn't exist — customers pay and see a 404. Additionally, there is NO mechanism to deliver the license key to the customer — the webhook generates it but nobody sends it anywhere.

**Approach:** Create a `get-license` Edge Function that looks up the license key by Stripe session ID (stored in the `licenses` table via `stripe_customer_id`). The checkout success page calls this function and displays the key directly — no email delivery needed for MVP. The Stripe payment link redirect URL must include `{CHECKOUT_SESSION_ID}` as a template variable.

- [ ] **Step 0: Update Stripe payment links to include session_id**

The current payment links redirect to `/checkout-success?tier=pro`. They need to include the Stripe session ID so we can look up the license. Stripe supports `{CHECKOUT_SESSION_ID}` as a template variable in redirect URLs. Recreate the payment links with the updated redirect URL, or update them via the Stripe API/Dashboard:

Redirect URL format: `https://mcpzerotrust.dev/checkout-success?tier=pro&session_id={CHECKOUT_SESSION_ID}`

- [ ] **Step 0b: Create get-license Edge Function**

Create `supabase/functions/get-license/index.ts` — a simple function that:
1. Accepts GET with `?session_id=cs_xxx`
2. Uses the Stripe API to look up the checkout session → get the customer ID
3. Queries the `licenses` table by `stripe_customer_id`
4. Returns the `license_key` if found, 404 if not yet generated (webhook may still be processing)
5. Requires no auth (the session_id is the auth — only someone who completed checkout has it)

Deploy: `supabase functions deploy get-license --no-verify-jwt`

- [ ] **Step 1: Create checkout-success.html**

Create `landing-page/checkout-success.html` with:
- Same dark theme styling as index.html (copy the CSS variables and base styles)
- Reads `tier` and `session_id` query parameters from URL
- Shows a confirmation message: "Welcome to MCP Zero-Trust Proxy [Pro/Enterprise]"
- Calls the `get-license` Edge Function with the session_id
- Displays the license key in a copyable code block when ready
- Shows a "Generating your license..." loading state if the webhook hasn't processed yet (poll every 3 seconds, max 30 seconds)
- Shows setup instructions: Docker command with the license key
- Has a support email link for help
- Falls back to "Contact support@mcpzerotrust.dev" if the key never appears

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Welcome — MCP Zero-Trust Proxy</title>
<style>
  :root {
    --navy: #0f1729;
    --accent: #3b82f6;
    --green: #10b981;
    --text: #e2e8f0;
    --text-muted: #94a3b8;
    --surface: #1e293b;
    --border: #334155;
    --white: #ffffff;
  }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
    background: var(--navy); color: var(--text); line-height: 1.6;
    -webkit-font-smoothing: antialiased;
    min-height: 100vh; display: flex; align-items: center; justify-content: center;
  }
  .container { max-width: 600px; margin: 0 auto; padding: 60px 24px; text-align: center; }
  .check-icon {
    width: 80px; height: 80px; border-radius: 50%;
    background: rgba(16,185,129,0.15); border: 2px solid var(--green);
    display: flex; align-items: center; justify-content: center;
    font-size: 40px; margin: 0 auto 32px;
  }
  h1 { font-size: 32px; font-weight: 800; margin-bottom: 16px; letter-spacing: -0.5px; }
  h1 span { color: var(--accent); }
  .desc { font-size: 18px; color: var(--text-muted); margin-bottom: 40px; line-height: 1.5; }
  .steps {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 12px; padding: 32px; text-align: left; margin-bottom: 32px;
  }
  .steps h2 { font-size: 18px; font-weight: 700; margin-bottom: 20px; }
  .steps ol { padding-left: 20px; }
  .steps li { padding: 8px 0; font-size: 15px; color: var(--text-muted); }
  .steps li strong { color: var(--text); }
  .steps code {
    background: rgba(59,130,246,0.1); padding: 2px 6px; border-radius: 4px;
    font-family: 'SF Mono', monospace; font-size: 13px; color: var(--accent);
  }
  .support {
    font-size: 14px; color: var(--text-muted);
  }
  .support a { color: var(--accent); text-decoration: none; }
  .support a:hover { text-decoration: underline; }
  .back-link {
    display: inline-block; margin-top: 32px; color: var(--text-muted);
    text-decoration: none; font-size: 14px;
  }
  .back-link:hover { color: var(--text); }
</style>
</head>
<body>
<div class="container">
  <div class="check-icon">✓</div>
  <h1>Welcome to <span id="tier-name">Pro</span></h1>
  <p class="desc">Your payment was successful. Your signed license key will be delivered to your email within a few minutes.</p>
  <div class="steps">
    <h2>What happens next</h2>
    <ol>
      <li><strong>Check your email</strong> — Your license key (a signed JWT) will arrive shortly. Check spam if you don't see it.</li>
      <li><strong>Add the key to your config</strong> — Set <code>license.key</code> in your config.yaml or pass it as <code>-e LICENSE_KEY=...</code> to Docker.</li>
      <li><strong>Restart the proxy</strong> — It will validate the key locally and unlock your tier's limits.</li>
    </ol>
  </div>
  <p class="support">
    Need help? Email <a href="mailto:support@mcpzerotrust.dev">support@mcpzerotrust.dev</a>
  </p>
  <a href="https://mcpzerotrust.dev" class="back-link">← Back to mcpzerotrust.dev</a>
</div>
<script>
  const params = new URLSearchParams(window.location.search);
  const tier = params.get('tier');
  const tierName = document.getElementById('tier-name');
  if (tier === 'enterprise') {
    tierName.textContent = 'Enterprise';
  } else {
    tierName.textContent = 'Pro';
  }
</script>
</body>
</html>
```

- [ ] **Step 2: Verify the page renders**

Open `landing-page/checkout-success.html?tier=pro` in a browser — should show "Welcome to Pro".
Open `landing-page/checkout-success.html?tier=enterprise` — should show "Welcome to Enterprise".

- [ ] **Step 3: Commit**

```bash
git add landing-page/checkout-success.html
git commit -m "feat(landing): add checkout success page for post-payment license delivery"
```

- [ ] **Step 4: Deploy to Vercel**

```bash
cd landing-page && vercel --prod --yes
```

Verify: `curl -sI https://mcpzerotrust.dev/checkout-success?tier=pro` should return 200.

---

## Task 3: Switch Stripe to Production Mode

**Prerequisites:** Andrew must have Stripe live mode enabled on his account (requires identity verification on Stripe dashboard). If not yet verified, this task blocks on that.

**Files:**
- Modify: `landing-page/index.html` (swap checkout links)
- Run: `scripts/setup-stripe-products.sh` (with live key)
- Modify: Supabase secrets (swap to live keys)

- [ ] **Step 1: Verify Stripe live mode is enabled**

```bash
stripe config --list
```

Check that `live_mode_api_key` exists and is not just `rk_live_***` (restricted key). You need a full `sk_live_` key. If you only have a restricted key, go to Stripe Dashboard → Developers → API Keys → Create a secret key.

- [ ] **Step 2: Run setup-stripe-products.sh with live key**

```bash
STRIPE_SECRET_KEY=sk_live_YOUR_KEY ./scripts/setup-stripe-products.sh
```

This will show a LIVE MODE warning and require typing "yes" to confirm. Save the output — you need the payment link URLs.

- [ ] **Step 3: Replace test checkout links in landing page**

In `landing-page/index.html`, replace the two `buy.stripe.com/test_*` URLs with the live payment link URLs from the script output.

- [ ] **Step 4: Update Supabase secrets with live Stripe key (NOT webhook secret yet)**

```bash
supabase secrets set \
  STRIPE_SECRET_KEY=sk_live_YOUR_KEY \
  STRIPE_PRICE_PRO=price_LIVE_PRO_ID \
  STRIPE_PRICE_ENTERPRISE=price_LIVE_ENTERPRISE_ID \
  --project-ref dwumoznjyckebuirghne
```

Note: Do NOT set `STRIPE_WEBHOOK_SECRET` yet — you don't have the live value until Step 5.

- [ ] **Step 4b: Verify LICENSE_SIGNING_KEY is set**

```bash
supabase secrets list --project-ref dwumoznjyckebuirghne 2>&1 | grep LICENSE_SIGNING_KEY
```

If missing, the license key generation will fail silently. Set it with:
```bash
supabase secrets set LICENSE_SIGNING_KEY="$(cat keys/private_key.pem)" --project-ref dwumoznjyckebuirghne
```

- [ ] **Step 5: Create live webhook endpoint**

```bash
curl -sS -X POST "https://api.stripe.com/v1/webhook_endpoints" \
  -u "sk_live_YOUR_KEY:" \
  -d "url=https://dwumoznjyckebuirghne.supabase.co/functions/v1/stripe-webhook" \
  -d "enabled_events[]=checkout.session.completed" \
  -d "enabled_events[]=customer.subscription.deleted" \
  -d "enabled_events[]=customer.subscription.updated"
```

Save the `whsec_` secret from the response. Update the Supabase secret:

```bash
supabase secrets set STRIPE_WEBHOOK_SECRET=whsec_LIVE_SECRET --project-ref dwumoznjyckebuirghne
```

- [ ] **Step 6: Delete the test webhook endpoint**

Go to Stripe Dashboard → Developers → Webhooks → delete the test endpoint pointing to Supabase.

- [ ] **Step 7: Redeploy Edge Functions and landing page**

Edge Functions need redeployment to pick up the new production secrets immediately:

```bash
supabase functions deploy stripe-webhook --no-verify-jwt
supabase functions deploy create-license --no-verify-jwt
supabase functions deploy get-license --no-verify-jwt
cd landing-page && vercel --prod --yes
git add landing-page/index.html
git commit -m "feat(billing): switch to production Stripe — live payment links"
git push origin main
```

- [ ] **Step 8: Test the live checkout flow end-to-end**

Use a real card (Stripe will charge $49 and you can refund immediately). Verify:
1. Checkout page loads (no "test mode" banner)
2. Payment succeeds
3. Redirect to `/checkout-success?tier=pro` works
4. License key appears in Supabase `licenses` table
5. Refund via Stripe Dashboard

---

## Task 4: Set Up Support Channel

**Files:**
- Modify: `landing-page/index.html` (add support email to footer)

- [ ] **Step 1: Decide on support email**

Options:
- **A) `support@mcpzerotrust.dev`** — professional, requires email forwarding setup on Vercel/domain provider
- **B) `andrewnoble1992@gmail.com`** — works immediately, less professional
- **C) Discord server** — community feel, visible to other users

Recommendation: Use option A (`support@mcpzerotrust.dev`) with email forwarding to your Gmail. Set up forwarding in Vercel Dashboard → Domain → Email.

If email forwarding isn't available on Vercel, use Cloudflare Email Routing (free) or simply use Gmail directly for now.

- [ ] **Step 2: Add support link to landing page footer**

In `landing-page/index.html`, update the footer:

```html
<!-- OLD -->
<footer>
  <div class="container">
    <p>MCP Zero-Trust Proxy &copy; 2026 &nbsp;·&nbsp; Built by developers, for developers who don't want to get breached.</p>
  </div>
</footer>
<!-- NEW -->
<footer>
  <div class="container">
    <p>MCP Zero-Trust Proxy &copy; 2026 &nbsp;·&nbsp; <a href="mailto:support@mcpzerotrust.dev" style="color: var(--text-muted); text-decoration: none;">support@mcpzerotrust.dev</a> &nbsp;·&nbsp; Built by developers, for developers who don't want to get breached.</p>
  </div>
</footer>
```

- [ ] **Step 3: Commit and deploy**

```bash
git add landing-page/index.html
git commit -m "feat(landing): add support email to footer"
cd landing-page && vercel --prod --yes
```

---

## Task 5: Prepare Show HN for Launch

**Files:**
- Modify: `docs/go-to-market/SHOW-HN.md`

- [ ] **Step 1: Decide open-source strategy**

This is a decision for Andrew, not code. The options:

**Option A (recommended): Open-source the core proxy, keep billing private.**
- Make the GitHub repo public
- The `internal/` Go code is the open-source part (auth, RBAC, proxy, audit, rate limiting)
- The `supabase/` Edge Functions and `scripts/` billing setup stay out of the open repo (or in a separate private repo)
- This is the standard "open core" model used by PostHog, Supabase, GitLab

**Option B: Keep private, offer code access on request.**
- Repo stays private
- Remove GitHub link from Show HN post
- Offer to share code with security researchers

Andrew picks one. Then proceed.

- [ ] **Step 2: Update Show HN post based on decision**

If Option A: Remove Option B paragraph, keep Option A paragraph. Update GitHub link.
If Option B: Remove Option A paragraph, keep Option B. Remove GitHub link from post.

- [ ] **Step 3: Fix the GitHub link**

If repo is public: link works as-is.
If repo is private: remove the `GitHub: https://github.com/...` line entirely. Don't link to a 404.

- [ ] **Step 3b: Fix the HN title — remove "5 min" claim**

Lines 1 and 3 of SHOW-HN.md still say "in 5 min". Update to match the landing page fix:
```markdown
Show HN: MCP Zero-Trust Proxy — drop-in auth for any MCP server
```

- [ ] **Step 3c: Fix "Immutable audit log" claim in post body**

Line 28 says "Immutable audit log" — but the audit log is not cryptographically immutable (no integrity verification). Update to match the corrected landing page:
```markdown
- Audit log — every MCP call logged: who, what, when, allowed/denied. Structured JSON export.
```

- [ ] **Step 4: Tighten the post**

Update the opening to lead with the one-liner:

```markdown
Hi HN,

I built a drop-in reverse proxy that adds zero-trust security to any MCP server:

    docker run -e MCP_TARGET=localhost:3000 ghcr.io/anoblescm/mcp-zero-trust-proxy

Why: MCP (Model Context Protocol) is how AI agents connect to tools — Claude, Cursor, Copilot all use it. But auth is optional in the spec. Result: 8,000+ exposed servers, 30+ CVEs in 60 days, and a CVSS 9.6 RCE in the most popular OAuth workaround (mcp-remote).
```

- [ ] **Step 5: Update test count**

Change "177+ tests" to "212 tests" (current count after security fixes).

- [ ] **Step 6: Add a clear ask**

Add at the end before the links:

```markdown
Looking for:
- Beta testers running MCP servers in production (free Pro access)
- Feedback on pricing and the open-core model
- Security researchers willing to review the auth/RBAC implementation
```

- [ ] **Step 7: Commit**

```bash
git add docs/go-to-market/SHOW-HN.md
git commit -m "docs(launch): finalize Show HN post for launch"
```

---

## Post-Plan: Launch Sequence

After all 5 tasks are complete, the launch sequence is:

1. `git push origin main` (push all fixes)
2. `git tag v1.0.0 && git push origin v1.0.0` (triggers Docker image + GitHub Release)
3. Wait for GitHub Actions to complete (~5 min)
4. Verify: `docker pull ghcr.io/anoblescm/mcp-zero-trust-proxy:v1.0.0`
5. Post Show HN (weekday 8-10am ET)
6. Execute community seeding checklist from SHOW-HN.md
