# MCP Zero-Trust Proxy — Claude Code Handoff

**Date:** March 19, 2026
**Owner:** Andrew Noble (andrewnoble1992@gmail.com)
**Domain:** mcpzerotrust.dev (purchased via Vercel, pending nameserver propagation)

---

## What This Project Is

A drop-in authentication middleware proxy for MCP (Model Context Protocol) servers. It sits in front of any MCP server and enforces OAuth 2.1 PKCE, RBAC, session isolation, rate limiting, and audit logging — with zero code changes to the protected server. Ranked #1 of 49 business ideas across 10 independent research reports with an 8.1/10 composite score.

## What's Already Done

### Research (in `/docs/research/`)
- **COMPETITIVE-ANALYSIS.md** — Full competitive landscape. 12 competitors mapped with feature gap matrix. Key finding: closest competitor (MCP Auth Proxy by sigbit) is free/drop-in but lacks RBAC, audit logging, session isolation. Enterprise players (Kong, Lunar, MintMCP) have those features but are complex and expensive. Our lane: simple deployment + enterprise features + transparent pricing.
- **ATTACK-SURFACE.md** — Threat landscape with named CVEs, breach timelines, exposed server counts. Key stats: 8,000+ exposed servers, 30+ CVEs in 60 days, CVSS 9.6 RCE in mcp-remote, Clawdbot breach hit 1,800+ servers with infostealers targeting within 48 hours.
- **EXECUTIVE-REPORT.md** — One-paragraph executive recommendation from the 49-idea synthesis.

### Go-to-Market (in `/docs/go-to-market/`)
- **OUTREACH-MESSAGES.md** — Pre-written templates: Discord/community posts (2 versions), DM templates (3 variants), Reddit posts (r/aiagents, r/SaaS), HN comment templates, post-call follow-up emails.

### Product Planning (in `/docs/`)
- **ROADMAP.md** — Full 90-day roadmap with 4 phases, weekly milestones, kill criteria, revenue projections, tech stack recommendation, parallel play timeline, and fallback plan.

### Landing Page (in `/landing-page/`)
- **index.html** — Complete single-file landing page. Dark theme, hero with Docker one-liner, threat stats banner, 6 feature cards, competitive comparison table, 3 pricing tiers, email capture form. Currently uses localStorage for form submission — **needs Supabase backend wired up**.

## What Needs To Be Done (In Order)

### Phase 0: Infrastructure (Do This First)

1. **Initialize git repo**
   ```bash
   cd mcp-zero-trust-proxy
   git init
   git add .
   git commit -m "Initial commit: research, landing page, and project docs"
   ```

2. **Create GitHub repo**
   ```bash
   gh repo create mcp-zero-trust-proxy --private --source=. --push
   ```

3. **Set up Supabase project**
   - Create new project: `mcp-zero-trust-proxy`
   - Create `waitlist` table:
     ```sql
     CREATE TABLE waitlist (
       id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
       email TEXT NOT NULL UNIQUE,
       source TEXT DEFAULT 'landing-page',
       created_at TIMESTAMPTZ DEFAULT now()
     );

     -- Enable RLS
     ALTER TABLE waitlist ENABLE ROW LEVEL SECURITY;

     -- Allow anonymous inserts only (for the landing page form)
     CREATE POLICY "Allow anonymous inserts" ON waitlist
       FOR INSERT TO anon
       WITH CHECK (true);

     -- Block reads from anon (admin only)
     CREATE POLICY "Admin read only" ON waitlist
       FOR SELECT TO authenticated
       USING (true);
     ```
   - Enable the `anon` key for client-side inserts
   - Note the project URL and anon key

4. **Wire landing page form to Supabase**
   - Replace the `handleSubmit` localStorage code in `landing-page/index.html` with:
     ```javascript
     const SUPABASE_URL = 'https://YOUR_PROJECT.supabase.co';
     const SUPABASE_ANON_KEY = 'YOUR_ANON_KEY';

     async function handleSubmit(e) {
       e.preventDefault();
       const email = document.getElementById('email-input').value;
       const btn = document.getElementById('submit-btn');
       const note = document.getElementById('form-note');

       btn.textContent = 'Joining...';
       btn.disabled = true;

       try {
         const res = await fetch(`${SUPABASE_URL}/rest/v1/waitlist`, {
           method: 'POST',
           headers: {
             'Content-Type': 'application/json',
             'apikey': SUPABASE_ANON_KEY,
             'Authorization': `Bearer ${SUPABASE_ANON_KEY}`,
             'Prefer': 'return=minimal'
           },
           body: JSON.stringify({ email, source: 'landing-page' })
         });

         if (res.ok) {
           btn.textContent = '✓ You\'re on the list!';
           btn.style.background = '#10b981';
           note.textContent = `We'll notify ${email} when the beta is ready.`;
           document.getElementById('email-input').disabled = true;
         } else if (res.status === 409) {
           btn.textContent = 'Already signed up!';
           btn.style.background = '#f59e0b';
           note.textContent = `${email} is already on the waitlist.`;
         } else {
           throw new Error('Signup failed');
         }
       } catch (err) {
         btn.textContent = 'Try again';
         btn.style.background = '#ef4444';
         btn.disabled = false;
         note.textContent = 'Something went wrong. Please try again.';
       }
     }
     ```
   - Store Supabase credentials as Vercel environment variables (not hardcoded)

5. **Deploy to Vercel**
   ```bash
   # Connect repo to Vercel
   vercel link

   # Set environment variables
   vercel env add NEXT_PUBLIC_SUPABASE_URL
   vercel env add NEXT_PUBLIC_SUPABASE_ANON_KEY

   # Deploy
   vercel --prod

   # Connect domain
   vercel domains add mcpzerotrust.dev
   ```

   Note: The landing page is static HTML so deployment is straightforward. For env vars in a static site, either inject them at build time or use a small API route.

### Phase 1: Validation Sprint (Days 1-7 of the Roadmap)

See `docs/ROADMAP.md` Phase 1 for the full day-by-day plan. The outreach messages in `docs/go-to-market/OUTREACH-MESSAGES.md` are ready to copy-paste.

### Phase 2: MVP Build (Days 8-21)

The actual proxy. Recommended tech stack (from ROADMAP.md):

- **Proxy core:** Go or Rust (single-binary, low latency)
- **Auth:** OAuth 2.1 PKCE built-in
- **Dashboard:** Next.js + Tailwind
- **Database:** SQLite (single-node) or Supabase/Postgres (cloud)
- **Billing:** Stripe
- **Deployment:** Docker + standalone binary
- **CI/CD:** GitHub Actions

The proxy code will live in `/src/`. Architecture decisions should be documented in `/docs/` as ADRs.

## Key Product Decisions (Already Made)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Pricing | $49 / $99 / $199 per month | Middle ground between free OSS (MCP Auth Proxy) and enterprise (Kong/Lunar) |
| Deployment | Docker + binary | Drop-in simplicity is our #1 differentiator |
| Auth standard | OAuth 2.1 PKCE | MCP spec standard, no proprietary lock-in |
| Self-hosted vs SaaS | Self-hosted first, SaaS later | Matches security buyer preference for data control |
| Target buyer | Independent devs, AI-native agencies, mid-market eng teams | Largest underserved segment per competitive analysis |

## Revenue Targets (from ROADMAP.md)

| Milestone | Timeline | Target |
|-----------|----------|--------|
| First beta users | Day 22 (Week 4) | 10+ teams |
| First revenue | Day 36 (Week 6) | $500+ MRR, 5+ paying |
| PMF checkpoint | Day 90 (Week 13) | $2K+ MRR, 100+ proxies, 10+ paying teams |
| Conservative Y1 | Month 12 | $3,236 MRR / $19,478 cumulative |
| Aggressive Y1 | Month 12 | $14,902 MRR / $68,008 cumulative |

## Kill Criteria (Check Weekly)

1. Fewer than 5 developers express real security concern by end of Week 1
2. Fewer than 3 people willing to pay $49+/mo after seeing the product by Week 4
3. Kong, Salt Security, or Cloudflare ships a turnkey MCP auth proxy
4. MCP protocol changes make proxy approach infeasible
5. MCP adoption stalls or Anthropic/OpenAI ship built-in auth

## File Structure

```
mcp-zero-trust-proxy/
├── HANDOFF.md              ← You are here
├── CLAUDE.md               ← Context file for Claude Code / AI tools
├── PROJECT-BRIEF.md        ← Product spec, architecture, and decisions
├── docs/
│   ├── ROADMAP.md          ← 90-day roadmap with phases and milestones
│   ├── research/
│   │   ├── COMPETITIVE-ANALYSIS.md
│   │   ├── ATTACK-SURFACE.md
│   │   └── EXECUTIVE-REPORT.md
│   └── go-to-market/
│       └── OUTREACH-MESSAGES.md
├── landing-page/
│   └── index.html          ← Complete landing page (needs Supabase wiring)
├── src/                    ← Proxy source code (to be built)
├── scripts/                ← Build/deploy scripts (to be created)
└── .github/                ← GitHub Actions workflows (to be created)
```
