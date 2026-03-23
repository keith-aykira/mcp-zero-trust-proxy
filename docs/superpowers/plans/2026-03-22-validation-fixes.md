# Validation Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the 2 blockers and 3 warnings found during E2E persona validation so the product is ready to ship.

**Architecture:** 5 independent tasks — 4 docs changes + 1 code change. All can be done in any order. No dependencies between tasks.

**Tech Stack:** Markdown (docs), Go (one code change in pipeline.go)

---

## File Structure

```
docs/QUICKSTART.md                    — Tasks 1, 3, 5 (add upgrade section, user_roles example, monitoring guide)
internal/proxy/pipeline.go            — Task 2 (improve 429 response message)
tests/e2e/marcus_test.go              — Task 2 (update test expectation)
docs/MULTI-TENANT.md                  — Task 4 (new file: agency/multi-tenant guide)
```

---

## Task 1: Add Upgrade Instructions to QUICKSTART.md

**Files:**
- Modify: `docs/QUICKSTART.md` (after the "License Key" section, before "Next steps")

**Why:** Marcus test `TestMarcus_DocsClarity` found no mention of "upgrade" anywhere. Users hitting free tier limits have zero guidance.

- [ ] **Step 1: Add the upgrade section**

Insert the following after line 311 (`If the license key is missing...`) and before the `---` / `## Next steps` section in `docs/QUICKSTART.md`:

```markdown

### Upgrading from Free to Pro

When you're ready for more:

1. Go to [mcpzerotrust.dev](https://mcpzerotrust.dev) and choose Pro ($49/mo) or Enterprise ($199/mo)
2. After checkout, you'll receive a license key (a signed JWT)
3. Add it to your config:

```yaml
license:
  key: "${LICENSE_KEY}"
```

4. Set the environment variable and restart:

```bash
# Binary
export LICENSE_KEY=your-license-key
mcpproxy --config ./config.yaml

# Docker
docker run -e LICENSE_KEY=your-license-key ...
```

The proxy validates the key locally (no network call) and unlocks your tier's limits immediately.

**What changes with Pro:**

| | Free | Pro |
|---|---|---|
| MCP servers | 1 | 5 |
| Rate limit | 10 req/min | 200 req/min |
| Audit | stdout only | file + rotation |
```

- [ ] **Step 2: Verify the upgrade keyword is now present**

```bash
grep -i "upgrade" docs/QUICKSTART.md
```

Expected: At least 2 matches (section title + content).

- [ ] **Step 3: Commit**

```bash
git add docs/QUICKSTART.md
git commit -m "docs: add upgrade instructions to QUICKSTART — fixes free→Pro path gap"
```

---

## Task 2: Improve 429 Rate Limit Response with Tier Info

**Files:**
- Modify: `internal/proxy/pipeline.go:284` and `internal/proxy/pipeline.go:294`
- Modify: `tests/e2e/marcus_test.go` (update rate limit message check)

**Why:** Marcus test `TestMarcus_FreeTierRateLimits` confirmed the 429 response is a generic "Rate limit exceeded" with no context about the tier or limit.

- [ ] **Step 1: Update the rate limit error messages in pipeline.go**

In `internal/proxy/pipeline.go`, change line 284:

```go
// OLD
writeJSONRPCError(w, nil, ErrCodeRateLimited, "Rate limit exceeded", http.StatusTooManyRequests)

// NEW
writeJSONRPCError(w, nil, ErrCodeRateLimited, "Rate limit exceeded. Upgrade your plan at https://mcpzerotrust.dev for higher limits.", http.StatusTooManyRequests)
```

And line 294 (the anonymous rate limit path):

```go
// OLD
writeJSONRPCError(w, nil, ErrCodeRateLimited, "Rate limit exceeded", http.StatusTooManyRequests)

// NEW
writeJSONRPCError(w, nil, ErrCodeRateLimited, "Rate limit exceeded. Upgrade your plan at https://mcpzerotrust.dev for higher limits.", http.StatusTooManyRequests)
```

- [ ] **Step 2: Run existing tests to verify no regressions**

```bash
export PATH="/Users/andrewnoble/.cache/pre-commit/repoj93vdc0b/golangenv-default/.go/bin:$PATH"
go test ./internal/proxy/ -v -timeout 30s
go test ./tests/integration/ -v -timeout 30s
```

Note: Some tests may check for the exact "Rate limit exceeded" message. If they fail, update the expected string to match the new message.

- [ ] **Step 3: Update marcus_test.go rate limit check**

In `tests/e2e/marcus_test.go`, the `TestMarcus_FreeTierRateLimits` function checks if the 429 body contains "upgrade" or "limit". With the new message, this should now pass as a "pass" finding instead of "gap". Verify by running:

```bash
go test ./tests/e2e/ -v -run "TestMarcus_FreeTierRateLimits" -timeout 30s
```

Expected: The test should now record a "pass" finding for "Rate limit error mentions limits".

- [ ] **Step 4: Commit**

```bash
git add internal/proxy/pipeline.go tests/e2e/marcus_test.go
git commit -m "fix: include upgrade URL in 429 rate limit response"
```

---

## Task 3: Add User-to-Role Mapping Example to QUICKSTART

**Files:**
- Modify: `docs/QUICKSTART.md` (in the "Configure RBAC" section)

**Why:** Priya and Sofia need to assign specific users to specific roles. The `user_roles.mapping` config exists but isn't shown in the QUICKSTART.

- [ ] **Step 1: Add user_roles mapping example**

In `docs/QUICKSTART.md`, replace line 210:

```markdown
To give a specific user a role, the OAuth token's associated email or sub claim is mapped to a role. Role assignment is done via the auth provider — typically by including the role in a custom claim or by mapping users to roles in your IdP.
```

With:

```markdown
**Mapping users to roles:**

Add a `user_roles` section to your config to assign roles by email address:

```yaml
user_roles:
  mapping:
    "alice@company.com": "admin"
    "bob@company.com": "readonly"
    "intern@company.com": "restricted"
  default: "readonly"  # role for authenticated users not in the mapping
```

Any authenticated user whose OAuth email matches a key gets that role. Users not in the mapping get the `default` role (defaults to `readonly` if not specified).
```

- [ ] **Step 2: Verify the example renders correctly**

```bash
grep -A 10 "user_roles" docs/QUICKSTART.md
```

- [ ] **Step 3: Commit**

```bash
git add docs/QUICKSTART.md
git commit -m "docs: add user-to-role mapping example to QUICKSTART RBAC section"
```

---

## Task 4: Add Multi-Tenant / Agency Setup Guide

**Files:**
- Create: `docs/MULTI-TENANT.md`
- Modify: `docs/QUICKSTART.md` (add link in "Next steps")

**Why:** Sofia's test `TestSofia_SingleUpstreamReality` confirmed config only supports one upstream. Agencies need documented architecture for multiple clients.

- [ ] **Step 1: Create MULTI-TENANT.md**

Create `docs/MULTI-TENANT.md`:

```markdown
# Multi-Client / Agency Setup

MCP Zero-Trust Proxy uses one upstream MCP server per proxy instance. If you manage MCP servers for multiple clients, run a separate proxy per client.

This is the recommended architecture for data isolation — each client gets its own proxy process, its own config, its own audit trail, and its own rate limits.

---

## Docker Compose Example

```yaml
version: "3.8"

services:
  proxy-client-a:
    image: ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
    ports: ["8081:8080"]
    volumes: ["./configs/client-a.yaml:/etc/mcpproxy/config.yaml"]
    environment:
      OAUTH_CLIENT_SECRET: ${CLIENT_A_OAUTH_SECRET}
      LICENSE_KEY: ${LICENSE_KEY}

  proxy-client-b:
    image: ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
    ports: ["8082:8080"]
    volumes: ["./configs/client-b.yaml:/etc/mcpproxy/config.yaml"]
    environment:
      OAUTH_CLIENT_SECRET: ${CLIENT_B_OAUTH_SECRET}
      LICENSE_KEY: ${LICENSE_KEY}

  proxy-client-c:
    image: ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
    ports: ["8083:8080"]
    volumes: ["./configs/client-c.yaml:/etc/mcpproxy/config.yaml"]
    environment:
      OAUTH_CLIENT_SECRET: ${CLIENT_C_OAUTH_SECRET}
      LICENSE_KEY: ${LICENSE_KEY}
```

Each client config points at a different upstream and has its own user-to-role mapping:

```yaml
# configs/client-a.yaml
server:
  upstream_url: "http://client-a-mcp:3000"

user_roles:
  mapping:
    "attorney@lawfirm.com": "admin"
    "paralegal@lawfirm.com": "readonly"
  default: "readonly"

audit:
  enabled: true
  output: "file"
  file_path: "/var/log/mcpproxy/client-a-audit.jsonl"
```

---

## Per-Client Audit Trails

Each proxy writes its own audit log. To separate audit trails by client:

- Set a different `audit.file_path` in each client's config
- Or use Docker log drivers to route each container's stdout to separate streams
- Audit entries include `client_id` (the authenticated user's email), so logs can also be filtered post-hoc

---

## Scaling

Each proxy instance is ~7MB memory at idle. For 10 clients, that's ~70MB total. The proxy adds <2ms p95 latency overhead.

A single Pro license key ($49/mo) covers up to 5 upstream servers across all instances. For more than 5, use Enterprise ($199/mo, unlimited).

---

## FAQ

**Can one proxy serve multiple upstreams?**
Not currently. The `upstream_url` config field accepts a single URL. One proxy = one upstream. This keeps the security model simple — each client's data is isolated at the process level.

**Can I share a license key across instances?**
Yes. The license key is validated locally (no network call). Use the same `LICENSE_KEY` environment variable for all instances.

**How do I know which client a request came from?**
The audit log includes `client_id` (the authenticated user's email). Each proxy instance only serves one client's upstream, so the source container also identifies the client.
```

- [ ] **Step 2: Add link to QUICKSTART.md "Next steps"**

In `docs/QUICKSTART.md`, add to the "Next steps" section (after line 320):

```markdown
- **Multi-client / agency setup:** [MULTI-TENANT.md](MULTI-TENANT.md) — how to run separate proxy instances per client with Docker Compose
```

- [ ] **Step 3: Commit**

```bash
git add docs/MULTI-TENANT.md docs/QUICKSTART.md
git commit -m "docs: add multi-tenant guide for agency deployments"
```

---

## Task 5: Add Monitoring Guide to QUICKSTART

**Files:**
- Modify: `docs/QUICKSTART.md` (add to "Next steps" section)

**Why:** Kai's test confirmed no `/metrics` endpoint. Need to document the interim monitoring approach via audit logs.

- [ ] **Step 1: Add monitoring guidance to "Next steps"**

In `docs/QUICKSTART.md`, add to the "Next steps" section:

```markdown
- **Monitoring:** The proxy exposes `/health` for uptime checks (returns `{"status":"ok"}`). For request-level metrics, parse the JSONL audit log with Filebeat, Fluentd, or Vector to feed Elasticsearch, Datadog, or Splunk. A Prometheus `/metrics` endpoint is on the roadmap.
```

- [ ] **Step 2: Commit**

```bash
git add docs/QUICKSTART.md
git commit -m "docs: add monitoring guidance to QUICKSTART — interim approach via audit logs"
```

---

## Execution Notes

- **All 5 tasks are independent** — can run in any order or parallel
- **Tasks 1, 3, 5 all modify QUICKSTART.md** — if running in parallel, serialize these three to avoid merge conflicts
- **Task 2 is the only code change** — requires running Go tests
- **Total estimated time:** ~45 minutes for all 5 tasks
