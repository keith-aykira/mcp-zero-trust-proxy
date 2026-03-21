# Show HN: MCP Zero-Trust Proxy — drop-in auth for AI tool servers in 5 min

**HN Title (< 80 chars):** `Show HN: MCP Zero-Trust Proxy — drop-in auth for AI tool servers in 5 min`

**URL:** https://mcpzerotrust.dev

---

## HN Post Body

Hi HN,

MCP (Model Context Protocol) is the emerging standard for connecting AI agents to tools and data — Claude, Cursor, Copilot, and others all support it. But authentication is optional in the spec. The result: 8,000+ exposed MCP servers with zero auth, 30+ CVEs in 60 days (Jan–Feb 2026), and the Clawdbot breach that hit 1,800+ servers in 48 hours.

The most popular OAuth workaround — mcp-remote — had a CVSS 9.6 RCE. Anthropic's own Git MCP server had prompt injection flaws that chained into full code execution. 75% of MCP servers are built by individuals with no centralized security review.

I built a drop-in reverse proxy that adds zero-trust security to any MCP server without touching the server code:

```
docker run -e MCP_TARGET=localhost:3000 ghcr.io/anoblescm/mcp-zero-trust-proxy
```

**What it does:**

- OAuth 2.1 PKCE authentication (GitHub, Google, Okta, any OIDC provider)
- Tool-level RBAC — restrict which tools each client can call, enforced at the proxy layer
- Session isolation — per-client execution boundaries prevent cross-tenant data leakage
- Immutable audit log — every MCP call logged: who, what, when, allowed/denied
- Rate limiting per client — token bucket, configurable per tier

**Technical details:**

- Written in Go — single binary, ~10MB, no runtime dependencies
- 177+ tests with -race flag (zero races)
- YAML config with `${ENV_VAR}` substitution
- License keys: ECDSA P-256 signed JWTs, validated locally with zero network calls (works air-gapped)
- MCP JSON-RPC 2.0 over HTTP+SSE (Streamable HTTP transport)
- Intercepts: `initialize`, `tools/list`, `tools/call`, `resources/read`, `prompts/get`

**Pricing:**

- Free: 1 server, 10 req/min, stdout audit — no license key needed
- Pro: $49/mo — 5 servers, 200 req/min, file audit with rotation
- Enterprise: $199/mo — unlimited everything, priority support

**On open-sourcing:**

*Option A (if open-sourcing at launch):* The core proxy is open source on GitHub. The licensing infrastructure (tier enforcement, billing integration) is the commercial layer. I want the security community to be able to audit the auth and RBAC implementation directly — security products shouldn't be black boxes.

*Option B (if staying private):* The repo is currently private while I work through the initial beta. I'll be opening it up once I've had a few teams run it in production and I'm confident in the security posture. Happy to share the code privately with security researchers — just ask.

Looking for feedback from anyone running MCP servers in production, especially:
- What auth solution (if any) you're using today
- What would make you trust a proxy sitting in your MCP traffic path
- Whether the pricing feels right for your use case

GitHub: https://github.com/AnobleSCM/mcp-zero-trust-proxy
Docs: https://mcpzerotrust.dev

---

## Community Seeding Checklist

Work through these in order on launch day. Post HN first (highest leverage), then seed the communities where your buyers live.

- [ ] Post to HN (weekday, 8–10am ET) — use the body above
- [ ] Post to r/aiagents — use Version A from [OUTREACH-MESSAGES.md](./OUTREACH-MESSAGES.md) (question-led, discussion starter)
- [ ] Post to r/SaaS — use the validation post from [OUTREACH-MESSAGES.md](./OUTREACH-MESSAGES.md) (building in public angle)
- [ ] Post to MCP Discord — use Version A from [OUTREACH-MESSAGES.md](./OUTREACH-MESSAGES.md) (question-led: "how are you securing your MCP servers?")
- [ ] Post to Claude Code Community Discord — same Version A template, adapted for Claude Code audience
- [ ] DM 10 developers who posted about MCP security — use Template A from [OUTREACH-MESSAGES.md](./OUTREACH-MESSAGES.md) ("saw your post about MCP security...")
- [ ] Tweet thread — lead with the breach data stats, end with the one-liner docker command and link

**Timing:** Post HN on a weekday between 8–10am ET for maximum front-page visibility. The other channels can follow throughout the same day.

**Response protocol:** Monitor HN and Reddit comments for 48 hours. Reply to technical questions with specifics. Don't be defensive about pricing or the closed-source question. Offer free Pro access to beta testers who engage.
