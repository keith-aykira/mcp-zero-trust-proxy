# MCP Zero-Trust Proxy — Registry Listing Drafts

Ready-to-submit listing content for MCP registries. Copy-paste during submission. Each section has all required fields for that registry.

## Submission Checklist

- [ ] Official MCP Registry submitted (registry.modelcontextprotocol.io)
- [ ] Smithery submitted (smithery.ai)
- [ ] awesome-mcp-servers PR opened (github.com/punkpeye/awesome-mcp-servers)

---

## 1. Official MCP Registry

**Registry URL:** https://registry.modelcontextprotocol.io

**Submission method:** Submit via the registry's contribution form or GitHub PR.

| Field | Value |
|-------|-------|
| **Name** | MCP Zero-Trust Proxy |
| **Category** | Security / Infrastructure |
| **URL** | https://github.com/AnobleSCM/mcp-zero-trust-proxy |
| **Docker image** | `ghcr.io/anoblescm/mcp-zero-trust-proxy` |
| **License** | MIT |

**Short description (1-2 sentences):**

> Free, open-source reverse proxy adding OAuth 2.1, RBAC, and audit logging to any MCP server. Zero code changes to the protected server.

**Long description:**

> A self-hosted reverse proxy that adds zero-trust security to any MCP server — no code changes required. Features include: OAuth 2.1 PKCE authentication (GitHub, Google, Okta, any OIDC provider), tool-level RBAC (admin/readonly/restricted roles), per-client sessions to prevent cross-tenant data leakage, structured JSON audit logging with file rotation, and per-client rate limiting with token bucket. Ships as a single Docker container (6.6MB) or Go binary (~10MB). MIT licensed. Supports MCP servers using HTTP+SSE transport (JSON-RPC 2.0).

**Note for reviewers:** This is a security infrastructure tool FOR MCP servers, not an MCP server itself. It sits in front of any existing MCP server and adds auth enforcement at the proxy layer. Recommend listing under Security or Infrastructure category.

---

## 2. Smithery

**Registry URL:** https://smithery.ai

**Submission method:** Submit via smithery.ai/submit or their GitHub-linked form.

| Field | Value |
|-------|-------|
| **Name** | MCP Zero-Trust Proxy |
| **Category** | Security |
| **URL** | https://github.com/AnobleSCM/mcp-zero-trust-proxy |
| **Install command** | `docker pull ghcr.io/anoblescm/mcp-zero-trust-proxy` |

**Short description:**

> Free, open-source reverse proxy adding OAuth 2.1 PKCE, tool-level RBAC, per-client sessions, and audit logging to any MCP server. Zero code changes required.

**Long description:**

> MCP Zero-Trust Proxy is a self-hosted security layer for any MCP server. It sits between your MCP clients (Claude, Cursor, Copilot) and your MCP servers, enforcing authentication and access control without requiring any modifications to the underlying server.
>
> Key features:
> - **OAuth 2.1 PKCE auth** — GitHub, Google, Okta, or any OIDC-compatible provider
> - **Tool-level RBAC** — admin, readonly, and restricted roles; restrict which tools each client can call
> - **Per-client sessions** — per-client execution boundaries prevent cross-tenant data leakage
> - **Structured audit logging** — every call logged (who, what, when, allowed/denied) in JSONL format with file rotation
> - **Rate limiting** — per-client token bucket (default: 300 req/min)
>
> Ships as a single Docker container (6.6MB image) or standalone Go binary (~10MB). Configuration is YAML with `${ENV_VAR}` substitution for secrets. MIT licensed. No external dependencies.

**Quick start:**

```bash
docker run \
  -e MCP_TARGET=localhost:3000 \
  -e AUTH_PROVIDER=github \
  -p 8080:8080 \
  ghcr.io/anoblescm/mcp-zero-trust-proxy
```

**Pricing:** Free and open source (MIT license)

---

## 3. awesome-mcp-servers (GitHub PR)

**Repo:** https://github.com/punkpeye/awesome-mcp-servers

**PR title:** `Add MCP Zero-Trust Proxy (security infrastructure)`

**Target section:** Security (or Infrastructure if Security section does not exist)

**Exact markdown line to add to README.md:**

```markdown
- [MCP Zero-Trust Proxy](https://github.com/AnobleSCM/mcp-zero-trust-proxy) - Open-source reverse proxy adding OAuth 2.1, RBAC, per-client sessions, and audit logging to any MCP server. Zero code changes. MIT licensed.
```

**PR body (copy-paste):**

---

**What:** MCP Zero-Trust Proxy is a free, open-source reverse proxy that adds OAuth 2.1 PKCE authentication, tool-level RBAC, per-client sessions, and structured audit logging to any MCP server — with zero code changes to the protected server.

**Why it belongs here:** As MCP adoption grows, authentication is becoming a critical gap. The proxy addresses a documented pain point (8,000+ exposed servers, OWASP MCP Top 10, 30+ CVEs in 60 days) and fills the category of security infrastructure specifically for MCP deployments.

**Checklist:**
- Link points to GitHub repo (MIT licensed, open source)
- Ships as Docker container (`ghcr.io/anoblescm/mcp-zero-trust-proxy`) and Go binary
- Supports HTTP+SSE MCP transport (JSON-RPC 2.0)
- 236+ tests passing (unit + integration + E2E)

---

**Tips for submission:**

- Open the PR from a fork of punkpeye/awesome-mcp-servers
- Reference any existing Security section — if there isn't one, propose adding it in the PR description
- Keep the PR description concise; reviewers merge many submissions

---

## Submission Timing

Submit all three registries on or shortly after launch day for maximum initial discoverability. The awesome-mcp-servers PR is highest leverage (most-starred MCP resource list on GitHub — likely to be crawled by AI search engines).

**Recommended order:** awesome-mcp-servers PR (highest traffic) → Smithery (active marketplace) → Official MCP Registry (authoritative, slower review cycle).
