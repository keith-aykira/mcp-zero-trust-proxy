# E2E Persona Validation — Design Spec

**Date:** 2026-03-22
**Phase:** 5 — E2E Validation (new phase, inserted before go-live)
**Goal:** Validate that the MCP Zero-Trust Proxy works end-to-end and delivers real value to 5 distinct buyer personas before tagging v1.0.0 and posting Show HN.

## Context

All code is complete. 212+ tests pass. Landing page built. Stripe billing wired (test mode). Edge Functions deployed. But nobody has walked the full journey — from "I have an MCP security problem" to "my servers are protected and I'm getting value." This phase is the quality gate before launch.

## Approach: Layer Cake

Three layers, each building on the last:

1. **Layer 1 — Build & Boot**: Shared foundation. Build the proxy, start a mock MCP server, verify the proxy is alive.
2. **Layer 2 — 5 Parallel Persona Runs**: Each persona runs their specific technical scenario against the live proxy with real HTTP requests.
3. **Layer 3 — Journey Evaluation**: Gemini expert panel roleplays each persona's non-technical experience (landing page, docs, pricing, day-2 retention), fed with real findings from Layer 2.

**Output:** Single consolidated report with ship/no-ship recommendation.

---

## Personas

### Persona 1: Marcus — Solo AI Dev

**Who:** Indie developer, builds side projects with Claude + MCP servers. Runs everything on his MacBook. Uses Homebrew, not Docker. Non-security expert — knows enough to be worried, not enough to roll his own.

**Trigger:** Just read on HN that `mcp-remote` has a CVSS 9.6 RCE. He uses that package. Googles "MCP server security" and finds mcpzerotrust.dev.

**Skeptic moment:** "$49/mo for a proxy? I'll just add a Bearer token check myself. ...Actually, what about RBAC and audit logs? Maybe the free tier first."

**Day 2 problem:** Running free tier for a week. Hits the 1-upstream limit. Wants to protect a second MCP server. Upgrade to Pro, or run two proxy instances?

### Persona 2: Priya — Startup CTO

**Who:** Technical co-founder at a 5-person AI startup. Team uses Claude and Cursor daily with 3 internal MCP servers (code search, docs, deployment). Just closed seed round. Knows Go and Docker.

**Trigger:** Lead investor asked "What's your AI security posture?" during due diligence. All 3 MCP servers are wide open on the internal network.

**Skeptic moment:** "Can I just use Cloudflare Zero Trust? ...Oh, it doesn't understand JSON-RPC or MCP tool-level permissions."

**Day 2 problem:** New engineer joins. Needs `readonly` on the docs server but `admin` on deployment. How to configure per-user, per-upstream role mappings?

### Persona 3: James — Security Engineer (Adversarial)

**Who:** Security engineer at a 200-person company. Reports to CISO. Runs Burp Suite before breakfast. Evaluates tools by trying to break them first.

**Trigger:** CISO forwarded the Palo Alto Unit 42 MCP advisory. "We have 12 MCP servers in production. Evaluate solutions by Friday."

**Skeptic moment:** "No SOC 2 report? No pentest results? I'll run my own assessment."

**Day 2 problem:** Needs to ship audit logs to Splunk. Docs say "structured JSON export" — but is there a way to push to an external collector?

### Persona 4: Sofia — Agency Dev

**Who:** Senior dev at an AI consultancy. Manages MCP deployments for 3 clients (law firm, healthcare startup, fintech). Each has strict data isolation requirements. Comfortable with Docker Compose and YAML.

**Trigger:** Client contract now requires SOC 2 controls on all AI tooling. Must prove Client A's traffic can't leak to Client B.

**Skeptic moment:** "The docs say 'session isolation' but I need tenant isolation — separate configs, separate audit trails. One proxy instance per client, or multi-tenant?"

**Day 2 problem:** Law firm client wants paralegals `readonly` but attorneys `admin`. Per-user roles within a single tenant — can the YAML handle it?

### Persona 5: Kai — DevOps Engineer

**Who:** Platform engineer at a Series B startup. Runs everything in Kubernetes. Cares about observability, uptime, not getting paged.

**Trigger:** Got paged at 3am because an MCP server showed up on Shodan. Needs to put it back behind a proxy in 2 hours.

**Skeptic moment:** "Where's the Prometheus /metrics endpoint? What happens to in-flight requests during a rolling update? Fail open or fail closed?"

**Day 2 problem:** Proxy running 3 weeks. Audit logs growing. Does rotation work? Can logs ship to ELK? Memory footprint under load? Goroutine leaks?

---

## Layer 1: Build & Boot

**Objective:** Get a running proxy + mock MCP server as a shared test foundation.

**Steps:**

1. Build the Go binary from source using the project's Go toolchain
2. Create a mock MCP server (Go httptest) that responds to:
   - `tools/list` — returns 5 tools with distinct names
   - `tools/call` — echoes back the tool name and arguments
   - `resources/list` / `resources/read` — returns test resources
   - `prompts/list` / `prompts/get` — returns test prompts
3. Write a test config YAML:
   - `upstream_url` pointing to mock server
   - Auth using a mock authenticator (Bearer token validation, no real OAuth)
   - 3 roles configured: admin (all tools), readonly (list only), restricted (specific tools)
   - User-to-role mapping: `marcus=admin`, `priya-admin=admin`, `priya-intern=readonly`, `james=admin`, `sofia-attorney=admin`, `sofia-paralegal=readonly`, `kai=admin`
   - Rate limit: 100/min sustained, 10 burst
   - Audit: file output to a temp directory
   - License: free tier (no key)
4. Start the proxy, verify `/health` returns 200
5. Make one successful `tools/list` request to confirm the pipeline works

**Success criteria:** Proxy running, accepting requests, forwarding to upstream, returning valid JSON-RPC responses.

---

## Layer 2: Persona Technical Runs

Each persona runs as a parallel agent with real curl/HTTP commands against the live proxy.

### Marcus — Solo AI Dev

**Focus:** Binary install, minimal config, free tier experience, upgrade friction.

**Scenario:**
1. Read QUICKSTART.md — can Marcus figure out what to do with only the docs?
2. Write a minimal config YAML from scratch (not copy the example — what would a first-timer write?)
3. Start the proxy with the binary
4. Make a successful `tools/list` request with a valid Bearer token
5. Make a successful `tools/call` request
6. Verify audit log file was created with the correct JSON-L format
7. Try to add a second upstream URL — hit the free tier limit
8. Read the error message — is it clear what to do next?
9. Check: are there instructions anywhere for upgrading from free to Pro?

**Findings template:**
- Setup time (minutes from "reading docs" to "first successful request")
- Config errors encountered and clarity of error messages
- Free tier limit messaging quality
- Upgrade path clarity
- Go/no-go from Marcus's perspective

### Priya — Startup CTO

**Focus:** Docker setup, multi-user RBAC, team management workflow.

**Scenario:**
1. Build Docker image from Dockerfile
2. Write a config with user-to-role mapping: `priya=admin`, `intern=readonly`
3. Start the proxy via Docker
4. As `priya` (admin): call `tools/list`, `tools/call` — both succeed
5. As `intern` (readonly): call `tools/list` — succeeds; call `tools/call` — denied
6. Verify the denial error message is clear (not a generic 403)
7. Verify audit log shows both the allowed and denied requests with correct user attribution
8. Simulate "add new team member": update the config YAML, restart proxy, verify new user works
9. Test with a Pro license key — verify tier limits expand (5 upstreams, 200 RPM)

**Findings template:**
- Docker build success and image size
- Config complexity for multi-user setup
- RBAC enforcement correctness
- Error message clarity for denied requests
- Config change workflow (restart required? graceful?)
- Go/no-go from Priya's perspective

### James — Security Engineer (Adversarial)

**Focus:** Break things. Find info leaks. Verify security claims.

**Scenario:**
1. **Malformed JSON-RPC:**
   - Missing `method` field
   - Null `params`
   - Empty body
   - Body exceeding max size limit (11MB if limit is 10MB)
   - Deeply nested JSON (100 levels)
   - Invalid JSON (syntax error)
   - Valid JSON but not JSON-RPC (missing jsonrpc field)
2. **RBAC bypass attempts:**
   - Call a denied tool with different casing (`Tools/Call` vs `tools/call`)
   - Add extra fields to the JSON-RPC request
   - Use a tool name that's a substring of an allowed tool
   - Send a batch request mixing allowed and denied tools
3. **Info leakage checks:**
   - Every error response checked for: stack traces, file paths, upstream URLs, provider config, internal IP addresses
   - Auth failure responses — do they reveal whether the token format is wrong vs expired vs unknown?
4. **Session isolation:**
   - Get a valid session as user A
   - Try to use user A's session token to access user B's resources
5. **Rate limiter stress:**
   - Send burst+1 requests simultaneously — verify 429
   - Try rotating Authorization headers to bypass per-client limiting
6. **Audit log integrity:**
   - Verify every request (including denied ones) appears in the audit log
   - Check that audit log entries can't be modified via the proxy's HTTP interface
   - Verify timestamps are monotonically increasing
7. **Header injection:**
   - Send requests with injected `X-Forwarded-For`, `X-Real-IP` headers
   - Verify the proxy doesn't blindly trust client-supplied headers

**Findings template:**
- Vulnerabilities found (critical / high / medium / low)
- Info leakage instances (with exact response snippets)
- RBAC bypass success/failure for each attempt
- Session isolation result
- Rate limiter bypass result
- Audit log completeness percentage
- Go/no-go from James's perspective (would he approve for CISO?)

### Sofia — Agency Dev

**Focus:** Multi-tenant isolation, per-client audit separation, config complexity.

**Scenario:**
1. Write a config for 3 "clients" — each with their own upstream MCP server:
   - Client A (law firm): `attorney=admin`, `paralegal=readonly`
   - Client B (healthcare): `doctor=admin`, `nurse=restricted`
   - Client C (fintech): `analyst=admin`
2. Start the proxy
3. As `attorney` (Client A): make requests — verify they reach Client A's upstream only
4. As `doctor` (Client B): make requests — verify they reach Client B's upstream only
5. As `paralegal` (Client A, readonly): try `tools/call` — verify denied
6. Check audit logs — are Client A's logs separable from Client B's? Can Sofia filter by client/upstream?
7. Test: does anything from Client A's requests appear in Client B's audit trail?
8. Evaluate: can this config scale to 10 clients? Is the YAML manageable?

**Note:** The current proxy architecture may support only a single upstream. If so, this persona's scenario reveals the gap — Sofia would need to run 3 separate proxy instances (one per client). Document whether this is a blocker or an acceptable architecture.

**Findings template:**
- Multi-upstream support (yes/no/workaround)
- Tenant isolation verification result
- Audit log separation capability
- Config complexity at scale
- Per-user roles within tenant feasibility
- Go/no-go from Sofia's perspective

### Kai — DevOps Engineer

**Focus:** Production readiness, monitoring, failure modes, resource consumption.

**Scenario:**
1. Build Docker image — check final image size (target: under 20MB)
2. Start proxy, hit `/health` — verify it returns structured JSON
3. Send 500 requests in 60 seconds — measure:
   - Response latency (p50, p95, p99)
   - Memory usage before and after
   - Goroutine count before and after (check for leaks)
4. Send a SIGTERM during active requests — verify:
   - In-flight requests complete (graceful shutdown)
   - New requests are rejected
   - Proxy exits with code 0
5. Generate enough audit log data to trigger file rotation — verify:
   - Old log file renamed correctly
   - New log file created
   - No log entries lost during rotation
6. Check: is there a `/metrics` or `/debug/vars` endpoint? (Probably not — document the gap)
7. Check: what happens if the upstream MCP server is down? Does the proxy return a clear error or hang?
8. Check: what happens if the config file has a syntax error? Does the proxy fail to start with a clear message?

**Findings template:**
- Image size
- Health check format and reliability
- Latency percentiles under load
- Memory delta after sustained traffic
- Goroutine leak check (count before/after)
- Graceful shutdown behavior
- Audit log rotation correctness
- Monitoring gaps (metrics endpoint, structured health)
- Upstream failure handling
- Config error messaging
- Go/no-go from Kai's perspective

---

## Layer 3: Journey Evaluation

**Method:** Feed Gemini expert panel the actual findings from Layer 2, then ask each persona:

1. **Landing page** — "You just found mcpzerotrust.dev because of [trigger event]. Read the landing page. What's your reaction? Would you keep reading or bounce? What questions do you have?"
2. **Docs** — "You decided to try it. Here's QUICKSTART.md. Can you get from zero to running? Where do you get stuck?"
3. **Purchase decision** — "You've been using the free tier for a week. Here's what you experienced: [Layer 2 findings]. Here's the pricing page. Would you pay? What's your objection?"
4. **Day 2 verdict** — "It's been a month. [Day 2 problem]. Would you renew? What would make you churn? What's the one thing that would make you upgrade?"

**Input to Gemini:** Each persona gets their Layer 2 findings + landing page HTML + QUICKSTART.md + pricing section + their specific trigger/skeptic/day-2 context.

---

## Output: Consolidated Report

**File:** `docs/e2e-validation/persona-validation-report.md`

### Structure:

1. **Executive Summary**
   - Ship / Ship with fixes / Do not ship
   - Top 3 blockers (if any)
   - Top 3 strengths

2. **Per-Persona Scorecard**

   | Persona | Setup OK? | Security OK? | Would Buy? | Would Renew? | Top Issue |
   |---------|-----------|-------------|------------|-------------|-----------|
   | Marcus  | | | | | |
   | Priya   | | | | | |
   | James   | | | | | |
   | Sofia   | | | | | |
   | Kai     | | | | | |

3. **Blockers** — Must fix before launch
4. **Warnings** — Won't stop launch but will cost customers
5. **Missing Features** — Things personas expected that don't exist
6. **Per-Persona Detail** — Full findings from Layer 2 + Layer 3 for each persona

---

## Success Criteria

- All 5 persona scenarios run to completion (even if they find failures)
- No critical security findings from James's adversarial testing
- At least 3 of 5 personas reach "would buy" or "would try free tier"
- All blockers documented with severity and fix estimate
- Report committed to repo

## Non-Goals

- This phase does NOT fix the issues found — it documents them
- This phase does NOT test real OAuth flows (uses mock authenticator)
- This phase does NOT test live Stripe checkout (test mode only)
- This phase does NOT deploy anything to production
