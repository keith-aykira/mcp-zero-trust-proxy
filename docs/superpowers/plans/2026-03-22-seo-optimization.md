# SEO & AI Discoverability Optimization — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all 9 SEO gaps so mcpzerotrust.dev ranks for "MCP server security" and shows rich previews on social/HN/Slack.

**Architecture:** 4 tasks. Task 1 adds all meta tags + structured data to index.html. Task 2 adds noindex to checkout-success.html. Task 3 creates static files (robots.txt, sitemap.xml, favicon, llms.txt). Task 4 updates vercel.json for headers.

**Tech Stack:** HTML, JSON-LD, Vercel

---

## File Structure

```
landing-page/
├── index.html              — Task 1 (meta tags, JSON-LD, favicon link)
├── checkout-success.html   — Task 2 (add noindex)
├── robots.txt              — Task 3
├── sitemap.xml             — Task 3
├── llms.txt                — Task 3
├── favicon.svg             — Task 3
└── vercel.json             — Task 4 (security headers, cache)
```

---

## Task 1: Add Meta Tags + Structured Data to index.html

**Files:**
- Modify: `landing-page/index.html` (inside `<head>`)

- [ ] **Step 1: Add Open Graph, Twitter, canonical, and favicon tags**

In `landing-page/index.html`, add the following after the `<meta name="description">` tag (line 7):

```html
<!-- Canonical -->
<link rel="canonical" href="https://mcpzerotrust.dev/">

<!-- Favicon -->
<link rel="icon" href="/favicon.svg" type="image/svg+xml">

<!-- Open Graph -->
<meta property="og:type" content="website">
<meta property="og:url" content="https://mcpzerotrust.dev/">
<meta property="og:title" content="MCP Zero-Trust Proxy — Secure Your MCP Servers in Minutes">
<meta property="og:description" content="Add authentication, access control, and audit logging to any MCP server. One Docker command, zero code changes.">
<meta property="og:site_name" content="MCP Zero-Trust Proxy">

<!-- Twitter -->
<meta name="twitter:card" content="summary">
<meta name="twitter:title" content="MCP Zero-Trust Proxy — Secure Your MCP Servers in Minutes">
<meta name="twitter:description" content="Add authentication, access control, and audit logging to any MCP server. One Docker command, zero code changes.">

<!-- Additional SEO -->
<meta name="robots" content="index, follow">
<meta name="author" content="MCP Zero-Trust Proxy">
<meta name="keywords" content="MCP, Model Context Protocol, security, proxy, OAuth, RBAC, audit, Claude, Cursor, Copilot, zero trust, authentication">
```

- [ ] **Step 2: Add JSON-LD structured data**

Add the following just before `</head>` in `landing-page/index.html`:

```html
<!-- Structured Data -->
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": "MCP Zero-Trust Proxy",
  "applicationCategory": "SecurityApplication",
  "operatingSystem": "Linux, macOS, Windows",
  "description": "A drop-in reverse proxy that adds OAuth 2.1 PKCE authentication, role-based access control, audit logging, and rate limiting to any MCP server — with zero code changes.",
  "url": "https://mcpzerotrust.dev",
  "offers": [
    {
      "@type": "Offer",
      "name": "Free",
      "price": "0",
      "priceCurrency": "USD",
      "description": "1 MCP server, 10 req/min, stdout audit"
    },
    {
      "@type": "Offer",
      "name": "Pro",
      "price": "49",
      "priceCurrency": "USD",
      "billingIncrement": "P1M",
      "description": "5 MCP servers, 200 req/min, file audit with rotation"
    },
    {
      "@type": "Offer",
      "name": "Enterprise",
      "price": "199",
      "priceCurrency": "USD",
      "billingIncrement": "P1M",
      "description": "Unlimited MCP servers, unlimited rate, all features"
    }
  ],
  "featureList": [
    "OAuth 2.1 PKCE Authentication",
    "Role-Based Access Control (RBAC)",
    "Structured JSON Audit Logging",
    "Per-Client Rate Limiting",
    "Session Isolation",
    "Docker Single-Container Deployment"
  ]
}
</script>
```

- [ ] **Step 3: Commit**

```bash
git add landing-page/index.html
git commit -m "feat(seo): add Open Graph, Twitter, canonical, JSON-LD structured data"
```

---

## Task 2: Add noindex to checkout-success.html

**Files:**
- Modify: `landing-page/checkout-success.html`

- [ ] **Step 1: Add noindex meta tag**

In `landing-page/checkout-success.html`, add inside `<head>` after the viewport meta tag:

```html
<meta name="robots" content="noindex, nofollow">
```

This prevents Google from indexing the post-payment page (which would be a dead end for search users).

- [ ] **Step 2: Commit**

```bash
git add landing-page/checkout-success.html
git commit -m "feat(seo): noindex checkout-success page — prevent indexing post-payment page"
```

---

## Task 3: Create Static SEO Files

**Files:**
- Create: `landing-page/robots.txt`
- Create: `landing-page/sitemap.xml`
- Create: `landing-page/llms.txt`
- Create: `landing-page/favicon.svg`

- [ ] **Step 1: Create robots.txt**

Create `landing-page/robots.txt`:

```
User-agent: *
Allow: /
Disallow: /checkout-success

Sitemap: https://mcpzerotrust.dev/sitemap.xml
```

- [ ] **Step 2: Create sitemap.xml**

Create `landing-page/sitemap.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://mcpzerotrust.dev/</loc>
    <lastmod>2026-03-22</lastmod>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
</urlset>
```

- [ ] **Step 3: Create llms.txt for AI search discoverability**

Create `landing-page/llms.txt`:

```
# MCP Zero-Trust Proxy

## What it does
MCP Zero-Trust Proxy is a drop-in reverse proxy that adds enterprise-grade security to any MCP (Model Context Protocol) server. It sits between MCP clients (Claude, Cursor, Copilot) and MCP servers, adding OAuth 2.1 PKCE authentication, role-based access control (RBAC), structured audit logging, per-client rate limiting, and session isolation — with zero code changes to the protected server.

## Key features
- OAuth 2.1 PKCE authentication (GitHub, Google, any OIDC provider)
- 3 built-in RBAC roles: admin, readonly, restricted
- Structured JSON audit logging with file rotation
- Per-client rate limiting (token bucket)
- Session isolation between clients
- Single Docker container deployment
- YAML-based configuration
- Sub-millisecond latency overhead (p50 < 400us)

## How to use it
1. Pull the Docker image: docker pull ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
2. Create a config.yaml with your upstream MCP server URL and OAuth provider
3. Run: docker run -p 8080:8080 -v ./config.yaml:/etc/mcpproxy/config.yaml ghcr.io/anoblescm/mcp-zero-trust-proxy
4. Point your MCP client at localhost:8080 instead of the MCP server directly

## Pricing
- Free: 1 MCP server, 10 req/min, stdout audit
- Pro ($49/mo): 5 MCP servers, 200 req/min, file audit with rotation
- Enterprise ($199/mo): Unlimited servers, unlimited rate, all features

## Links
- Website: https://mcpzerotrust.dev
- Documentation: https://github.com/AnobleSCM/mcp-zero-trust-proxy/blob/main/docs/QUICKSTART.md
- Docker image: ghcr.io/anoblescm/mcp-zero-trust-proxy
```

- [ ] **Step 4: Create favicon.svg**

Create `landing-page/favicon.svg` — a simple shield icon matching the product's security theme:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <path d="M16 2L4 8v8c0 7.7 5.1 14.9 12 16 6.9-1.1 12-8.3 12-16V8L16 2z" fill="#3d5a8a"/>
  <path d="M14 17.4l-3.7-3.7 1.4-1.4 2.3 2.3 5.3-5.3 1.4 1.4L14 17.4z" fill="#fff"/>
</svg>
```

- [ ] **Step 5: Commit all static files**

```bash
git add landing-page/robots.txt landing-page/sitemap.xml landing-page/llms.txt landing-page/favicon.svg
git commit -m "feat(seo): add robots.txt, sitemap.xml, llms.txt, favicon"
```

---

## Task 4: Update vercel.json with Security Headers and Cache

**Files:**
- Modify: `landing-page/vercel.json`

- [ ] **Step 1: Update vercel.json**

Replace the contents of `landing-page/vercel.json` with:

```json
{
  "cleanUrls": true,
  "headers": [
    {
      "source": "/(.*)",
      "headers": [
        { "key": "X-Content-Type-Options", "value": "nosniff" },
        { "key": "X-Frame-Options", "value": "DENY" },
        { "key": "Referrer-Policy", "value": "strict-origin-when-cross-origin" }
      ]
    },
    {
      "source": "/favicon.svg",
      "headers": [
        { "key": "Cache-Control", "value": "public, max-age=604800, immutable" }
      ]
    }
  ]
}
```

- [ ] **Step 2: Commit**

```bash
git add landing-page/vercel.json
git commit -m "feat(seo): add security headers and cache policy to vercel.json"
```

---

## Execution Notes

- **All 4 tasks are independent** — can run in any order
- **No code changes** — all HTML/static files
- **No tests to run** — these are SEO/meta changes
- **After all tasks:** redeploy to Vercel with `cd landing-page && vercel --prod --yes`
- **Verify after deploy:**
  - `curl -sI https://mcpzerotrust.dev/ | grep -i "og:\|twitter:\|canonical"` (meta tags won't show in headers but will be in HTML)
  - `curl https://mcpzerotrust.dev/robots.txt` should return the robots file
  - `curl https://mcpzerotrust.dev/sitemap.xml` should return the sitemap
  - `curl https://mcpzerotrust.dev/llms.txt` should return the AI-readable description
  - Test OG preview at opengraph.xyz or by pasting the URL in Slack/Discord
