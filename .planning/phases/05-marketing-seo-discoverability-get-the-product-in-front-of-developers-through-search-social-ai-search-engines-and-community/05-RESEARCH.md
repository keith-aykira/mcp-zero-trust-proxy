# Phase 5: Marketing, SEO & Discoverability — Research

**Researched:** 2026-03-22
**Domain:** Developer marketing, SEO, AI search optimization, community distribution
**Confidence:** HIGH (core channels), MEDIUM (AI search ranking specifics)

## Summary

MCP Zero-Trust Proxy is a developer security tool entering a narrow market window (3-6 months before consolidation). All product is built — this phase is entirely about distribution: getting the right developers to find, evaluate, and buy the proxy. The product has strong hooks (breach data, CVE wave, 8,000+ exposed servers) and a concrete one-command install. The goal is to compound discoverability across every channel where security-conscious MCP developers find tools.

The marketing challenge is specific: the buyer is not a broad developer audience but a narrow segment — teams deploying MCP servers in production who need auth, RBAC, and audit logging. Every channel choice should be evaluated against "does this reach someone running MCP servers in production?" rather than developer audience size alone.

A critical constraint: the GitHub repo stays private for now, which limits "link to GitHub" HN tactics. The Show HN post draft already exists (`docs/go-to-market/SHOW-HN.md`). The existing landing page has OG tags, JSON-LD, robots.txt, sitemap.xml, and llms.txt already implemented. Phase 5 extends from this foundation.

**Primary recommendation:** Launch sequentially: Show HN first (highest leverage, proven draft exists) — then seed MCP-specific community channels — then build comparison pages targeting competitor keywords — then expand to distribution listings (MCP registries, awesome lists). Content marketing and AI search optimization are ongoing activities that compound over weeks.

---

## What Already Exists (Do Not Rebuild)

| Asset | Location | Status |
|-------|----------|--------|
| OG tags, JSON-LD, robots.txt | `landing-page/index.html`, `robots.txt` | Done (commit 9b0affe) |
| sitemap.xml | `landing-page/sitemap.xml` | Single-page only |
| llms.txt | `landing-page/llms.txt` | Done (commit 9b0affe) |
| Show HN post | `docs/go-to-market/SHOW-HN.md` | Draft ready, not posted |
| Community outreach templates | `docs/go-to-market/OUTREACH-MESSAGES.md` | 5 templates ready |
| Pain signals / breach data | `docs/research/PAIN-SIGNALS.md` | 14 signals, direct quotes |
| Competitive analysis | `docs/research/COMPETITIVE-ANALYSIS.md` | 12+ competitors mapped |

---

## Standard Stack

### Core Distribution Channels

| Channel | Format | Audience Fit | Priority |
|---------|--------|--------------|----------|
| Hacker News (Show HN) | Single post + comment engagement | High — Go, security, infra devs | P1 — launch day |
| MCP Community Discord | Question-led post + follow-ups | Very high — direct MCP builders | P1 — launch day |
| r/aiagents | Discussion thread | High — AI agent builders | P1 — launch day |
| MCP Registry (official) | Listing submission | Very high — developers searching for tools | P1 — launch day |
| Smithery | Tool listing | High — MCP marketplace | P1 — launch day |
| Glama | Tool listing | High — curated MCP directory | P2 — week 1 |
| awesome-mcp-servers | PR submission | High — discovery by builders | P2 — week 1 |
| Product Hunt | Full launch | Medium — broad dev audience | P2 — week 2 |
| Dev.to | Technical tutorial post | Medium — dev community SEO | P3 — ongoing |
| r/SaaS | Validation/building-in-public post | Low-medium — founder audience | P3 — week 2 |
| Twitter/X | Thread with breach data | Medium — dev security discourse | P3 — ongoing |
| LinkedIn | Technical post | Low-medium — B2B buyers | P4 — ongoing |

### SEO Assets to Build

| Asset | Purpose | Keyword Target | Effort |
|-------|---------|----------------|--------|
| Comparison page: vs sigbit | High-intent "vs" keyword | "MCP auth proxy alternative" | Low |
| Comparison page: vs Kong | High-intent enterprise alternative | "Kong MCP gateway alternative" | Low |
| Comparison page: vs Cloudflare | Vendor lock-in angle | "Cloudflare MCP alternative self-hosted" | Low |
| FAQ schema (landing page) | AI search citations + rich results | "how to secure MCP server" | Low |
| Blog: "Securing Your MCP Server in 5 Minutes" | Tutorial SEO + AI search | "secure MCP server OAuth" | Medium |
| Blog: "OWASP MCP Top 10 — What It Means" | OWASP alignment, news hook | "OWASP MCP security" | Medium |
| Blog: "30 CVEs in 60 Days" writeup | Breach-data SEO angle | "MCP security vulnerabilities 2026" | Medium |
| Updated sitemap.xml | Index new pages | — | Trivial |

### Supporting Tools

| Tool | Purpose | When |
|------|---------|------|
| Google Search Console | Verify indexing, monitor rankings | Week 1 — verify submission |
| JSON-LD SoftwareApplication schema | Rich results, AI citations | Week 1 — enhance landing page |
| FAQ schema markup | AI overview appearance | Week 1 — add to landing page |
| GitHub Topics tags | GitHub discovery (when repo goes public) | Pre-public launch |

---

## Architecture Patterns

### Distribution Sequence Pattern

The evidence-based sequence for a developer security tool launch:

```
Day 1: Show HN → MCP Discord → r/aiagents → r/SaaS
Week 1: Registries (MCP.io, Smithery, Glama) → awesome-mcp-servers PR
Week 2: Product Hunt launch → Dev.to tutorial post → Twitter/X thread
Week 3+: Blog content (SEO compounding) → comparison pages → DM outreach
Ongoing: Comment on relevant HN/Reddit threads → content refreshes
```

**Rationale:** HN has the highest spike potential for developer tools and sets the narrative. MCP community channels reach the exact buyer. Registries provide passive ongoing discovery. Content marketing and SEO compound over time but require weeks before returning results.

### Hacker News Show HN Pattern

**Title format (verified from HN guidelines):** `Show HN: [Product] — [one-line value prop]`

The existing draft title is already well-formed: `Show HN: MCP Zero-Trust Proxy — drop-in auth for any MCP server`

**Post structure (from HN official guidelines and launch analysis):**
1. What it does (one sentence)
2. Technical specifics (Go devs care about implementation)
3. The problem / why it exists (breach data hook)
4. How to try it (friction-free)
5. Honest pricing
6. What you're looking for (feedback, beta testers)

**Critical constraints:**
- No friends/employees upvoting — HN ring detection is strong
- Reply to ALL comments within first 2 hours — engagement signals
- Never defensive about pricing or open-source questions
- Timing: Tuesday–Thursday, 8–10am ET (verified from multiple sources)
- The existing draft leads with breach data — correct approach
- The repo is private — acknowledge this honestly ("currently private beta, email for code access")

### MCP Community Discord Pattern

Official MCP Contributor Discord: `https://discord.gg/6CSzBmMkjX` (11,752+ members)
The community Discord: `https://discord.gg/model-context-protocol-1312302100125843476`

**Important constraint:** The MCP Contributor Discord explicitly prohibits product/service marketing. Community channels are for contributors, not promotional posts.

**Correct approach:** Engage as a builder contributing to the security conversation. Use the question-led Version A template from `OUTREACH-MESSAGES.md` — "how are you securing your MCP servers?" — rather than a product announcement. Reference OWASP MCP Top 10, the CVE wave. Let the product come up organically when asked.

**Alternative channels:** Look for general AI dev communities (Claude Code Community Discord, Cursor Discord) where MCP is discussed but no explicit anti-marketing rules apply.

### MCP Registry Listing Pattern

Listing in directories provides passive, ongoing organic discovery. Priority listings:

1. **Official MCP Registry** (`registry.modelcontextprotocol.io`) — 87 servers, authoritative
2. **Smithery** — marketplace, searchable by category including security
3. **Glama** (`glama.ai/mcp/servers`) — curated, 10,000 servers, security badge review
4. **PulseMCP** — 14,274+ servers, searchable
5. **MCP.so** — large collection, Claude MCP integration
6. **awesome-mcp-servers** (`punkpeye/awesome-mcp-servers`) — GitHub PR submission

**Note:** These are tool/server listings, not security proxy listings. Position it as an MCP security tool that wraps any MCP server. Category: "Security" or "Infrastructure."

### Comparison Page SEO Pattern

Comparison pages ("X vs Y") are bottom-of-funnel, high buying intent, and typically less competitive than head keywords.

**URL structure:**
```
mcpzerotrust.dev/compare/vs-kong/
mcpzerotrust.dev/compare/vs-cloudflare/
mcpzerotrust.dev/compare/vs-mcp-auth-proxy/
```

**Page structure (verified from founderpath.com analysis):**
1. H1: "[Product] vs [Competitor] — [year]"
2. Quick verdict (1 sentence — be honest)
3. Feature comparison table (checkboxes)
4. Pricing comparison
5. Use case guidance ("choose [competitor] if...", "choose us if...")
6. Customer testimonials or social proof
7. CTA

**Key insight:** Acknowledge competitor strengths honestly. The audience is in evaluation mode and responds better to transparent analysis than pure advocacy. The unique position — drop-in + RBAC + audit + transparent pricing — is defensible and checkable.

### Content Marketing / Blog Pattern

Blog posts serve two purposes simultaneously: SEO (traditional search rankings) and GEO (AI search citations). Content that gets cited by AI systems tends to have:
- Clear logical heading hierarchy (H1 → H2 → H3, single H1 per page)
- Three or more schema types (Article, FAQ, HowTo where applicable)
- Regular updates (pages not updated quarterly are 3x more likely to lose AI citations)
- Authoritative statistics with source attribution

**High-priority blog topics ranked by search demand + effort:**

1. **"How to Secure Your MCP Server: OAuth 2.1 in 5 Minutes"** — direct tutorial, captures "secure MCP server" searches
2. **"OWASP MCP Top 10 — What Every Developer Should Know"** — news hook, OWASP brand authority
3. **"MCP Security in 2026: The CVE Wave and What It Means"** — data-heavy, shareable, AI-citation worthy
4. **"Zero Trust for AI Agents: Why MCP Auth Is Not Enough"** — conceptual explainer, links to product
5. **"Self-Hosting MCP Servers Securely: A Practical Guide"** — how-to, targets "self-host MCP server" searches

**Publishing platform options:**
- Landing page blog (`mcpzerotrust.dev/blog/`) — preferred (builds domain authority)
- Dev.to cross-post — reaches developer audience, canonical URL set to mcpzerotrust.dev
- Medium cross-post — optional secondary reach

**Current landing page limitation:** It's a single static HTML file. Adding a blog requires either static site generation (Hugo, Astro) or a simple `/blog/` subdirectory with individual HTML pages. The latter is lower effort and consistent with the existing Vercel static setup.

### AI Search Optimization (GEO) Pattern

AI search engines (ChatGPT, Perplexity, Claude, Gemini) cite content differently from traditional search engines.

**What drives AI citations (from GEO research):**
- Pages with logical heading hierarchies have 2.8x higher citation rates
- 61% of cited pages use 3+ schema types
- Content freshness matters: 70%+ of cited pages updated within 12 months
- llms.txt already implemented — good foundation
- Structured JSON-LD already on landing page — good foundation

**What's missing for better AI citations:**
- FAQ schema markup on landing page
- `SoftwareApplication` JSON-LD (currently has basic structured data — needs review)
- Authoritative content (blog posts with citations) on the domain
- Backlinks from authoritative developer/security sources

**The llms.txt already exists** at `landing-page/llms.txt` — it correctly describes the product, key features, pricing, and links. The gap is that there's only one page for AI to find. More indexed content (comparison pages, blog posts) = more surface area for AI citations.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Blog CMS | Custom blog engine | Static HTML pages in `/blog/` subfolder, or simple Astro/Hugo site | Existing Vercel static setup is sufficient; full CMS is months of scope |
| Link tracking | Custom UTM system | UTM parameters on all outreach links (`?utm_source=hn&utm_medium=launch`) | Zero cost, Google Analytics / Vercel Analytics reads them natively |
| SEO analytics | Custom ranking tracker | Google Search Console (free) | No-code, authoritative, indexes directly |
| Email capture | New form | Existing Supabase waitlist form on landing page | Already wired and working |
| Social scheduling | Custom tool | Manual posting — low volume, high context needed | Not worth automation at this stage |

---

## Common Pitfalls

### Pitfall 1: Posting to MCP Contributor Discord as a Promotion
**What goes wrong:** Post gets deleted, account gets flagged, relationship with MCP core team damaged.
**Why it happens:** The official MCP Contributor Discord explicitly prohibits service/product marketing. This is enforced.
**How to avoid:** Use the question-led approach (Version A template). Never lead with the product. Contribute genuinely to the security conversation, then let the product come up when asked.
**Warning signs:** Drafting a post that starts with "I built..." rather than "How are you...?"

### Pitfall 2: Show HN Timing Error
**What goes wrong:** Post at wrong time, gets buried immediately, never gains momentum.
**Why it happens:** HN is heavily time-zone dependent. Non-peak posts don't surface.
**How to avoid:** Post Tuesday–Thursday, 8–10am ET (not PT — ET is correct based on where most HN readers are). Do not post Friday, Saturday, Sunday, or Monday.
**Warning signs:** Scheduling a post for a weekend or late afternoon.

### Pitfall 3: Comparison Page Bias Destroys Trust
**What goes wrong:** Comparison page is transparently one-sided, readers dismiss it, no SEO benefit.
**Why it happens:** Founders write comparison pages as sales documents rather than evaluation aids.
**How to avoid:** Acknowledge competitor strengths explicitly. "Choose sigbit if you only need auth and don't need RBAC or audit logs — it's free and drop-in." This builds credibility and actually helps conversion by being the honest comparison buyers are looking for.
**Warning signs:** Every competitor row is a check-minus.

### Pitfall 4: Over-Seeding Community Channels
**What goes wrong:** Multiple posts in same community in quick succession looks spammy, gets reported, damages reputation.
**Why it happens:** Founder wants maximum reach day 1.
**How to avoid:** One post per community channel per launch window. Space out follow-up comments by at least 1 week. The OUTREACH-MESSAGES.md templates are correctly scoped.
**Warning signs:** Posting the same message to 5 channels on the same day.

### Pitfall 5: Blog Without Sitemap Update
**What goes wrong:** New blog pages are not indexed by Google or AI crawlers.
**Why it happens:** sitemap.xml is manually maintained and only has the root URL.
**How to avoid:** Update sitemap.xml every time a new page is published. Add each new URL with `<lastmod>` date.
**Warning signs:** Publishing a blog post and forgetting to update sitemap.xml.

### Pitfall 6: Private Repo Friction on HN
**What goes wrong:** HN commenters ask about source code, get told it's private, disengage or post skeptical comments that hurt vote momentum.
**Why it happens:** HN audience strongly favors open-source tools. The private repo is a known friction point.
**How to avoid:** The existing Show HN draft addresses this well — "currently private while I work through the initial beta. Happy to share the code privately with security researchers." This is the right framing. Do not apologize for it — explain the reasoning (security review process before open-sourcing) and offer the private access path.
**Warning signs:** Removing the private repo acknowledgment from the post draft.

### Pitfall 7: Registry Listings as Proxy Servers (Wrong Category)
**What goes wrong:** MCP registries list MCP servers (tools that provide capabilities). The proxy is not an MCP server itself — it protects MCP servers.
**Why it happens:** Trying to list in every directory without checking the category fit.
**How to avoid:** For MCP registries, position it as a security wrapper/infrastructure tool for MCP server operators, not as an MCP server end-users connect to. Some registries have Infrastructure or Security categories — use those. If no appropriate category exists, skip that registry.
**Warning signs:** Listing the proxy under "data access" or "coding" categories.

---

## Code Examples

### UTM Link Structure
```
# HN launch
https://mcpzerotrust.dev/?utm_source=hackernews&utm_medium=show-hn&utm_campaign=launch-2026

# Reddit r/aiagents
https://mcpzerotrust.dev/?utm_source=reddit&utm_medium=community&utm_campaign=launch-2026&utm_content=aiagents

# MCP Discord
https://mcpzerotrust.dev/?utm_source=discord&utm_medium=community&utm_campaign=launch-2026&utm_content=mcp-official

# Product Hunt
https://mcpzerotrust.dev/?utm_source=producthunt&utm_medium=launch&utm_campaign=ph-2026
```

### FAQ Schema Markup (Add to `<head>` in index.html)
```json
// Source: schema.org/FAQPage
{
  "@context": "https://schema.org",
  "@type": "FAQPage",
  "mainEntity": [
    {
      "@type": "Question",
      "name": "How do I secure my MCP server with OAuth?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "Run docker run -e MCP_TARGET=your-server:3000 ghcr.io/anoblescm/mcp-zero-trust-proxy in front of any MCP server to add OAuth 2.1 PKCE authentication, RBAC, and audit logging with zero code changes."
      }
    },
    {
      "@type": "Question",
      "name": "What is the difference between MCP Zero-Trust Proxy and MCP Auth Proxy?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "MCP Auth Proxy (sigbit) is free and adds OAuth authentication only. MCP Zero-Trust Proxy adds OAuth plus tool-level RBAC, session isolation, and structured audit logging — features required for compliance and multi-tenant deployments."
      }
    },
    {
      "@type": "Question",
      "name": "Is MCP Zero-Trust Proxy self-hosted?",
      "acceptedAnswer": {
        "@type": "Answer",
        "text": "Yes. MCP Zero-Trust Proxy is a self-hosted single Docker container or standalone Go binary. Your data never leaves your infrastructure."
      }
    }
  ]
}
```

### SoftwareApplication Schema (Verify/Enhance in index.html)
```json
// Source: schema.org/SoftwareApplication
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "MCP Zero-Trust Proxy",
  "description": "Drop-in reverse proxy adding OAuth 2.1 PKCE authentication, RBAC, session isolation, and audit logging to any MCP server. Zero code changes required.",
  "applicationCategory": "SecurityApplication",
  "operatingSystem": "Linux, macOS, Windows",
  "offers": [
    {
      "@type": "Offer",
      "name": "Free",
      "price": "0",
      "priceCurrency": "USD"
    },
    {
      "@type": "Offer",
      "name": "Pro",
      "price": "49",
      "priceCurrency": "USD",
      "billingPeriod": "P1M"
    },
    {
      "@type": "Offer",
      "name": "Enterprise",
      "price": "199",
      "priceCurrency": "USD",
      "billingPeriod": "P1M"
    }
  ],
  "url": "https://mcpzerotrust.dev",
  "downloadUrl": "https://github.com/keith-aykira/mcp-zero-trust-proxy/releases"
}
```

### Comparison Page HTML Structure
```html
<!-- Source: founderpath.com analysis + schema.org -->
<!-- File: landing-page/compare/vs-sigbit/index.html -->
<head>
  <title>MCP Zero-Trust Proxy vs MCP Auth Proxy (sigbit) — 2026 Comparison</title>
  <meta name="description" content="Compare MCP Zero-Trust Proxy and MCP Auth Proxy. Feature-by-feature breakdown of OAuth, RBAC, audit logging, pricing, and deployment.">
</head>
<body>
  <h1>MCP Zero-Trust Proxy vs MCP Auth Proxy — 2026</h1>
  <p>Quick verdict: [honest one-liner]</p>
  <!-- Feature table -->
  <!-- Pricing comparison -->
  <!-- "Choose X if..." guidance -->
  <!-- CTA -->
</body>
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SEO only (Google) | SEO + GEO (AI search) | 2024–2025 | Must optimize for ChatGPT/Perplexity citations, not just Google rankings |
| robots.txt only for crawlers | robots.txt + llms.txt + AI-specific schemas | 2024 | llms.txt already implemented — AI crawlers use this |
| Product Hunt as primary launch | HN first for dev tools, PH secondary | 2023–present | HN has higher quality traffic for developer security tools; PH is volume/awareness |
| Single-channel launch | Multi-channel sequenced seeding | Current best practice | HN spike + community followthrough + passive registry discovery = sustained traffic |
| Generic comparison pages | Honest evaluation-style comparison pages | Current best practice | Buyers distrust pure advocacy; honest pages convert better and rank better |

**Deprecated/outdated:**
- LinkedIn video posts: LinkedIn deprioritized video in 2025; carousels/text perform better now
- "One big launch day" strategy: Best results come from sequenced launch over 2-3 weeks, not a single moment

---

## Open Questions

1. **Private vs public repo timing**
   - What we know: Repo stays private for launch per CLAUDE.md. HN audience prefers open-source. Existing Show HN draft acknowledges this well.
   - What's unclear: Does the repo go public at any point during this phase? If so, when?
   - Recommendation: Plan for the current state (private). Add a task to update Show HN response templates if repo goes public mid-phase.

2. **Blog infrastructure**
   - What we know: Landing page is a single static HTML file. Vercel deployment. Adding a blog requires new pages.
   - What's unclear: Does Andrew want a simple `/blog/` subfolder with static HTML files, or a proper static site generator (Astro, Hugo)?
   - Recommendation: Static HTML subfolder (`landing-page/blog/`) is lowest effort and sufficient. No build tools, no dependencies. Each post is an `index.html` in its own folder. Consistent with existing stack.

3. **MCP Registry category fit**
   - What we know: MCP registries primarily list MCP servers (tools that provide capabilities). The proxy is not an MCP server.
   - What's unclear: Whether official MCP Registry or Smithery has an "infrastructure" or "security" category for protective tools.
   - Recommendation: Check each registry during task execution. If no suitable category, skip that registry and note it. The awesome-mcp-servers list is better suited (it includes infrastructure tools).

4. **Anthropic/Cursor/GitHub Copilot developer relations**
   - What we know: Target buyers are Claude, Cursor, Copilot users. Anthropic and Microsoft have developer relations channels.
   - What's unclear: Whether there's an official Anthropic partner/listing program for MCP security tools.
   - Recommendation: Research this during execution. If Anthropic has a "MCP ecosystem" or "partner" page, a listing there would be high value.

---

## Validation Architecture

> nyquist_validation is enabled per .planning/config.json

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (existing, 236+ tests passing) |
| Config file | N/A — marketing phase, no new Go code |
| Quick run command | `go test ./...` (existing tests, verify nothing broken) |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

Marketing work is not unit-testable in the traditional sense. Validation for this phase is manual verification of published assets:

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MKT-01 | Show HN post published | manual | Check https://news.ycombinator.com | N/A |
| MKT-02 | Landing page indexed by Google | manual | Google Search Console verification | N/A |
| MKT-03 | FAQ schema valid | smoke | `npx schema-markup-validator` or Google Rich Results Test | N/A |
| MKT-04 | New pages in sitemap | smoke | `grep -c "<url>" landing-page/sitemap.xml` | ❌ needs update |
| MKT-05 | Comparison pages live and accessible | smoke | `curl -o /dev/null -s -w "%{http_code}" https://mcpzerotrust.dev/compare/vs-sigbit/` | ❌ Wave 0 |
| MKT-06 | UTM links functional | manual | Click tracked links, verify in analytics | N/A |
| MKT-07 | Registry listings live | manual | Check each registry URL | N/A |

### Sampling Rate
- **Per task commit:** Verify the specific deliverable (page loads, schema validates)
- **Per wave merge:** Smoke test all new URLs respond 200
- **Phase gate:** Show HN posted, at least 3 registries listed, at least 2 comparison pages live

### Wave 0 Gaps
- [ ] `landing-page/compare/vs-sigbit/index.html` — comparison page (MKT-05)
- [ ] `landing-page/compare/vs-kong/index.html` — comparison page (MKT-05)
- [ ] `landing-page/blog/` — blog post infrastructure (content SEO)
- [ ] sitemap.xml — update to include new pages as they're added (MKT-04)

---

## Sources

### Primary (HIGH confidence)
- HN official Show HN guidelines: https://news.ycombinator.com/showhn.html — post format, community rules
- MCP official community page: https://modelcontextprotocol.io/community/communication — channel rules, anti-marketing policy
- MCP Registry (official): https://registry.modelcontextprotocol.io/ — listing status, categories
- Existing Phase 1 research: `docs/research/PAIN-SIGNALS.md`, `COMPETITIVE-ANALYSIS.md`, `BUYER-BEHAVIOR.md` — pain signals, buyer segments, competitor landscape

### Secondary (MEDIUM confidence)
- markepear.dev HN launch guide — tactics verified with HN guidelines and multiple corroborating sources
- founderpath.com comparison page SEO — verified with multiple SEO sources
- Glama MCP directory (glama.ai/mcp/servers) — 10,000 servers, security category exists
- awesome-mcp-servers GitHub (punkpeye/awesome-mcp-servers) — active, accepts PRs
- GEO research: foundationinc.co, frase.io — AI search citation patterns verified across multiple sources

### Tertiary (LOW confidence)
- Twitter/X content strategy data: LinkedIn and X algorithm changes — single source reports, verify timing before acting
- Product Hunt launch strategy for 2026: pattern consistent with 2025 but specific algorithm details change frequently

---

## Metadata

**Confidence breakdown:**
- Standard channels: HIGH — MCP community channels verified, HN timing verified from multiple independent sources, registry existence confirmed
- Architecture patterns: HIGH — sequenced launch approach is well-documented pattern for dev tools
- GEO/AI search: MEDIUM — llms.txt standard exists and is implemented, AI citation factors are documented but rapidly evolving
- Comparison pages: HIGH — bottom-of-funnel SEO is well-established; honest comparison page approach verified
- Blog infrastructure approach: HIGH — static HTML on Vercel is consistent with existing stack

**Research date:** 2026-03-22
**Valid until:** 2026-04-22 (30 days — stable domain; AI search specifics may evolve faster)
