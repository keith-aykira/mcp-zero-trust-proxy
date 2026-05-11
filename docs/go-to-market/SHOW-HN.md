# Show HN: Open-source zero-trust proxy for MCP servers (Go)

**HN Title (< 80 chars):** `Show HN: Open-source zero-trust proxy for MCP servers (Go)`

**URL:** https://github.com/keith-aykira/mcp-zero-trust-proxy

---

## HN Post Body

Hi HN,

I built a drop-in reverse proxy that adds zero-trust security to any MCP server. MIT-licensed, single Go binary, no code changes to the protected server:

```
docker run -e MCP_TARGET=localhost:3000 ghcr.io/anoblescm/mcp-zero-trust-proxy
```

**What it does:**

- OAuth 2.1 PKCE authentication (GitHub, Google, Okta, any OIDC provider)
- Tool-level RBAC — restrict which MCP tools each user can call, enforced at the proxy layer
- Per-client sessions — separate rate limit buckets and audit identity per authenticated user
- Structured audit logging — every MCP call logged as JSONL: who, what, when, allowed/denied
- Rate limiting — per-client token bucket, configurable in YAML

**Why this exists:**

MCP (Model Context Protocol) is how AI agents connect to tools — Claude, Cursor, Copilot all use it. Auth is optional in the spec. Result: 8,000+ exposed servers, 30+ CVEs in 60 days, and a CVSS 9.6 RCE in the most popular OAuth workaround (mcp-remote). The Clawdbot breach hit 1,800+ servers in 48 hours.

**Technical details:**

- Go single binary, ~10MB, no runtime dependencies
- 236+ tests passing (go test -race, zero data races detected)
- YAML config with `${ENV_VAR}` substitution
- MCP JSON-RPC 2.0 over HTTP+SSE (Streamable HTTP transport)
- Intercepts: `initialize`, `tools/list`, `tools/call`, `resources/read`, `prompts/get`
- Pipeline: body size check -> auth -> rate limit -> JSON-RPC parse -> RBAC -> forward -> filter response -> audit

**Looking for:**

- Contributors (Go, security, docs — all welcome)
- Security researchers willing to review the auth and RBAC implementation
- Feedback on the architecture and approach

GitHub: https://github.com/keith-aykira/mcp-zero-trust-proxy

---

## Community Seeding Checklist

Work through these in order on launch day. Post HN first (highest leverage), then seed the communities where your users live.

- [ ] Post to HN (weekday, 8-10am ET) — use the body above
- [ ] Post to r/aiagents — question-led discussion starter: "How are you securing your MCP servers in production?" Link to repo in context.
- [ ] Post to r/golang — technical angle: "Open-sourced a zero-trust proxy for MCP servers — Go, single binary, 236+ tests." Focus on the Go implementation details.
- [ ] Post to MCP Discord — "How are you handling auth for MCP servers?" Share the repo as one approach.
- [ ] Post to Claude Code Community Discord — same framing, adapted for Claude Code audience.
- [ ] DM 10 developers who posted about MCP security — "saw your post about MCP security, built an open-source proxy that might help."
- [ ] Tweet thread — lead with the breach stats, end with the docker one-liner and GitHub link.

**Timing:** Post HN on a weekday between 8-10am ET for maximum front-page visibility. The other channels can follow throughout the same day.

**Response protocol:** Monitor HN and Reddit comments for 48 hours. Reply to technical questions with specifics and code references. Point people to the repo, QUICKSTART.md, and the test suite. Welcome contributions openly.

---

## Draft Answers for Hard HN Questions

### 1. "Why not Caddy + Authelia?"

Those are general-purpose reverse proxies with general-purpose auth. They work well for HTTP endpoints. This proxy is purpose-built for MCP JSON-RPC: it parses `tools/list` and `tools/call` at the protocol level, enforces per-tool RBAC (e.g., user X can call `read_file` but not `delete_database`), and filters tool lists by role in the response. A generic proxy can't do tool-level access control without custom plugins that understand MCP's JSON-RPC schema.

### 2. "How do rate limits work across replicas?"

Per-instance token bucket. This is an honest limitation — there's no cross-instance coordination. For most deployments (1-3 instances), the per-instance limits are sufficient. If you need globally coordinated rate limiting, put a Redis-backed rate limiter in front, or contribute a Redis backend — the rate limiter interface is pluggable.

### 3. "What does per-client sessions actually mean?"

Each authenticated user gets their own rate limit bucket and audit identity, tracked via their OAuth bearer token. The proxy maps token to user, user to role, role to allowed tools. It is not OS-level process isolation or sandboxing — it's application-level session tracking with per-user enforcement.

### 4. "How do I monitor upstream failures?"

The audit log captures every request including failures with status codes and error details, written as structured JSONL. Parse it with Filebeat, Fluentd, or Vector into your observability stack. A Prometheus `/metrics` endpoint is on the roadmap — PRs welcome.

### 5. "What's your bus factor?"

One, honestly. The code is MIT-licensed, has 236+ tests, and the architecture is straightforward Go — anyone who writes Go can read and maintain it. The docs cover config, architecture, and deployment. PRs and co-maintainers are welcome.

### 6. "What does this do that nginx + OAuth2 Proxy doesn't?"

MCP-aware request parsing. nginx + OAuth2 Proxy gives you authentication at the HTTP layer, but it has no visibility into the JSON-RPC payload. This proxy understands `tools/list` and `tools/call` methods and can filter or block at the individual tool level. It also produces audit logs that track which specific MCP tools were called by which user — not just which HTTP endpoints were hit.

### 7. "What happens when Anthropic adds native auth?"

The MCP spec now includes authentication, but it only covers the authentication piece — verifying identity. This proxy adds authorization (who can call which tools), audit logging (structured record of every call), and rate limiting (per-client throttling). The spec has no plans to standardize RBAC, audit, or rate limiting. If native auth becomes universal, this proxy still handles the authorization and observability layer on top of it.
