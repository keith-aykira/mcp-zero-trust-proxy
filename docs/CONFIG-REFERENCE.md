# Configuration Reference

Complete reference for `config.yaml`. All fields shown with types, defaults, and examples.

---

## Complete example

```yaml
server:
  upstream_url: "http://localhost:3000"
  listen_addr: ":8080"

auth:
  provider: "github"
  client_id: "your-client-id"
  client_secret: "${OAUTH_CLIENT_SECRET}"
  redirect_url: "http://localhost:8080/auth/callback"

roles:
  - name: "admin"
    allowed_tools: []
    deny_tools: []
  - name: "readonly"
    allowed_tools: []
    deny_tools: []
  - name: "restricted"
    allowed_tools: ["read_file", "search"]
    deny_tools: []

rate_limit:
  requests_per_minute: 100
  burst_size: 10

audit:
  enabled: true
  output: "stdout"
  file_path: ""

logging:
  level: "info"
  format: "json"
```

---

## Minimal config

The only required field is `server.upstream_url`. All other fields have defaults:

```yaml
server:
  upstream_url: "http://localhost:3000"
```

With only this, the proxy:
- Listens on `:8080`
- Has no authentication (all requests pass through)
- Uses the three built-in roles with default permissions
- Rate-limits to 100 req/min with burst of 10
- Writes audit logs to stdout
- Logs at INFO level in JSON format

---

## Environment variable substitution

Any string value in the config supports `${ENV_VAR}` substitution. The substitution happens before YAML parsing:

```yaml
auth:
  client_secret: "${OAUTH_CLIENT_SECRET}"  # reads from environment at startup
```

If the variable is unset, the literal string `${ENV_VAR}` is used (no error). Use this for all secrets — never hardcode credentials in the config file.

---

## server

Controls where the proxy listens and where it forwards requests.

### server.upstream_url

| | |
|---|---|
| **Type** | string |
| **Required** | yes |
| **Default** | none |

The URL of the upstream MCP server. All authenticated, authorized requests are forwarded here. Must include scheme and host.

```yaml
server:
  upstream_url: "http://localhost:3000"
  # upstream_url: "http://mcp-server.internal:8000"
  # upstream_url: "https://mcp-server.example.com"
```

The proxy preserves the request path when forwarding. A request to `http://proxy:8080/` is forwarded to `http://upstream:3000/`.

### server.listen_addr

| | |
|---|---|
| **Type** | string |
| **Required** | no |
| **Default** | `":8080"` |

The TCP address the proxy HTTP server listens on. Standard Go net/http address format: `[host]:port`.

```yaml
server:
  listen_addr: ":8080"          # all interfaces, port 8080 (default)
  # listen_addr: "127.0.0.1:8080"  # localhost only
  # listen_addr: ":9000"            # custom port
```

---

## auth

Controls OAuth 2.1 PKCE authentication. If omitted or empty, all requests pass through without authentication (no-auth mode — only appropriate for local development).

### auth.provider

| | |
|---|---|
| **Type** | string |
| **Required** | yes (if auth section present) |
| **Default** | `"github"` (fallback if empty) |
| **Values** | `"github"`, `"google"`, `"oidc"` |

The OAuth 2.1 PKCE provider to use for authentication.

```yaml
auth:
  provider: "github"
  # provider: "google"
  # provider: "oidc"
```

### auth.client_id

| | |
|---|---|
| **Type** | string |
| **Required** | yes (if auth section present) |
| **Default** | none |

The OAuth application client ID from your provider's developer console.

```yaml
auth:
  client_id: "Iv1.abc123def456"       # GitHub format
  # client_id: "123456789-abc.apps.googleusercontent.com"  # Google format
  # client_id: "your-oidc-client-id"  # OIDC format
```

### auth.client_secret

| | |
|---|---|
| **Type** | string |
| **Required** | yes (if auth section present) |
| **Default** | none |

The OAuth application client secret. Always use `${ENV_VAR}` substitution — never hardcode this value.

```yaml
auth:
  client_secret: "${OAUTH_CLIENT_SECRET}"
```

### auth.issuer_url

| | |
|---|---|
| **Type** | string |
| **Required** | yes when `provider: "oidc"` |
| **Default** | none |

The OIDC issuer URL. The proxy appends standard OIDC paths (e.g., `/authorize`, `/token`) to build the OAuth endpoints.

```yaml
auth:
  provider: "oidc"
  issuer_url: "https://your-org.okta.com"
  # issuer_url: "https://your-domain.auth0.com"
  # issuer_url: "https://accounts.example.com"
```

Not used for `provider: "github"` or `provider: "google"` — those providers have hardcoded endpoints.

### auth.redirect_url

| | |
|---|---|
| **Type** | string |
| **Required** | yes (if auth section present) |
| **Default** | none |

The OAuth callback URL registered with your provider. Must be an exact match for what you registered in the provider's developer console.

```yaml
auth:
  redirect_url: "http://localhost:8080/auth/callback"
  # redirect_url: "https://proxy.example.com/auth/callback"
```

The proxy serves this callback at `/auth/callback` automatically.

---

## roles

Defines the RBAC permissions for each role. Three roles are always available: `admin`, `readonly`, `restricted`. If this section is omitted, default permissions are applied.

### Role definitions

Each entry in the `roles` list has:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Role name. Must be one of: `"admin"`, `"readonly"`, `"restricted"` |
| `allowed_tools` | []string | Tool names this role may call via `tools/call`. Empty list = all tools allowed (admin). |
| `deny_tools` | []string | Tool names explicitly denied. Deny rules take precedence over allow rules. |

### Default role behavior

**admin:**
- All MCP methods allowed: `initialize`, `tools/list`, `tools/call`, `resources/read`, `prompts/get`, etc.
- All tools allowed (no restrictions on tool names)
- `allowed_tools: []` means unrestricted

**readonly:**
- Can call: `initialize`, `tools/list`, `resources/read`, `prompts/get`
- Cannot call `tools/call` (all tool calls are denied regardless of tool name)
- Sees all tools in `tools/list` response

**restricted:**
- Can call `tools/call` but only for tools in `allowed_tools`
- `tools/list` response is filtered to only show the allowed tools
- If `allowed_tools` is empty, no tools can be called

### Example role configurations

```yaml
roles:
  # Admin: full access (default behavior with empty allowed_tools)
  - name: "admin"
    allowed_tools: []
    deny_tools: []

  # Readonly: list and read, no tool execution
  - name: "readonly"
    allowed_tools: []
    deny_tools: []

  # Restricted: read-only filesystem access
  - name: "restricted"
    allowed_tools:
      - "read_file"
      - "list_directory"
      - "search_files"
    deny_tools: []
```

**Denying specific tools for admin:**

```yaml
roles:
  - name: "admin"
    allowed_tools: []
    deny_tools:
      - "delete_table"
      - "drop_database"
```

`deny_tools` takes precedence over `allowed_tools`. An admin with `deny_tools: ["dangerous_tool"]` cannot call `dangerous_tool` even though `allowed_tools` is empty (all allowed).

---

## rate_limit

Per-client request rate limiting using a token bucket algorithm. Limits apply per authenticated client ID.

### rate_limit.requests_per_minute

| | |
|---|---|
| **Type** | integer |
| **Required** | no |
| **Default** | `100` |

Sustained request rate per client, in requests per minute. The token bucket refills at this rate.

```yaml
rate_limit:
  requests_per_minute: 100   # 100 req/min sustained (default)
  # requests_per_minute: 60  # 1 req/second
  # requests_per_minute: 600 # 10 req/second
```

### rate_limit.burst_size

| | |
|---|---|
| **Type** | integer |
| **Required** | no |
| **Default** | `10` |

Maximum burst of requests a client may send above the sustained rate. The token bucket starts full (with `burst_size` tokens). Requests that exceed the burst return HTTP 429 with JSON-RPC error code `-32003`.

```yaml
rate_limit:
  burst_size: 10   # can send 10 requests instantly, then 100/min sustained (default)
  # burst_size: 50  # larger burst for clients with bursty patterns
```

---

## audit

Controls the immutable audit trail. Every request (allowed and denied) is logged as a JSONL record.

### audit.enabled

| | |
|---|---|
| **Type** | boolean |
| **Required** | no |
| **Default** | `true` |

Enables or disables audit logging. Default is `true` — strongly recommended to keep enabled in production.

```yaml
audit:
  enabled: true   # (default)
  # enabled: false  # disable only in development
```

### audit.output

| | |
|---|---|
| **Type** | string |
| **Required** | no |
| **Default** | `"stdout"` |
| **Values** | `"stdout"`, `"file"`, `"both"` |

Where audit log entries are written.

- `"stdout"` — standard output; works well with Docker and log aggregators (Splunk, Datadog, CloudWatch)
- `"file"` — writes to `audit.file_path`; requires `file_path` to be set
- `"both"` — writes to both stdout and file

```yaml
audit:
  output: "stdout"  # (default)
  # output: "file"
  # output: "both"
```

### audit.file_path

| | |
|---|---|
| **Type** | string |
| **Required** | yes when `output: "file"` or `output: "both"` |
| **Default** | `""` |

Path to the audit log file. The proxy appends to this file; it does not rotate logs (use logrotate or a log aggregator for rotation).

```yaml
audit:
  output: "file"
  file_path: "/var/log/mcpproxy/audit.jsonl"
```

---

## logging

Controls the proxy's operational log output (startup messages, errors, request processing). Separate from the audit log.

### logging.level

| | |
|---|---|
| **Type** | string |
| **Required** | no |
| **Default** | `"info"` |
| **Values** | `"debug"`, `"info"`, `"warn"`, `"error"` |

Minimum log level to emit. `"debug"` is verbose (includes per-request details). `"error"` is quiet (only errors).

```yaml
logging:
  level: "info"    # (default)
  # level: "debug"  # verbose — shows request routing decisions
  # level: "warn"   # only warnings and errors
  # level: "error"  # only errors
```

### logging.format

| | |
|---|---|
| **Type** | string |
| **Required** | no |
| **Default** | `"json"` |
| **Values** | `"json"`, `"text"` |

Log output format.

- `"json"` — structured JSON (default); best for production and log aggregators
- `"text"` — human-readable with colors; best for local development

```yaml
logging:
  format: "json"    # (default)
  # format: "text"  # human-readable for development
```

---

## Audit log output format

Each audit log entry is a single-line JSON object (JSONL format). One entry per request.

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string (RFC3339) | UTC timestamp when the request arrived at the proxy |
| `request_id` | string | Unique ID for this request; use to correlate proxy logs with upstream logs |
| `client_id` | string | Authenticated client identifier (typically email or OAuth subject) |
| `session_id` | string | OAuth session ID; empty for unauthenticated requests |
| `method` | string | MCP JSON-RPC method name (e.g., `"tools/call"`, `"tools/list"`) |
| `tool_name` | string | Tool name for `tools/call` requests; empty for all other methods |
| `allowed` | boolean | `true` if the request was proxied to upstream; `false` if denied |
| `denied_reason` | string | Human-readable denial reason; empty when `allowed: true` |
| `latency_ms` | integer | Request processing time in nanoseconds (Go duration) |

### Example entries

Allowed `tools/call`:
```json
{"timestamp":"2025-01-15T10:30:00Z","request_id":"abc123XY","client_id":"user@example.com","session_id":"sess-456","method":"tools/call","tool_name":"read_file","allowed":true,"latency_ms":12450000}
```

Denied due to RBAC:
```json
{"timestamp":"2025-01-15T10:30:01Z","request_id":"def456AB","client_id":"user@example.com","session_id":"sess-456","method":"tools/call","tool_name":"execute_command","allowed":false,"denied_reason":"rbac: tool \"execute_command\" is not in the allowed list for role \"restricted\"","latency_ms":1200000}
```

Denied due to missing auth:
```json
{"timestamp":"2025-01-15T10:30:02Z","request_id":"ghi789CD","client_id":"","session_id":"","method":"tools/list","allowed":false,"denied_reason":"auth: missing Authorization header","latency_ms":500000}
```

Denied due to rate limit:
```json
{"timestamp":"2025-01-15T10:30:03Z","request_id":"jkl012EF","client_id":"user@example.com","session_id":"sess-456","method":"tools/call","tool_name":"search","allowed":false,"denied_reason":"rate limit exceeded","latency_ms":800000}
```

---

## Special routes

These routes bypass the security pipeline:

| Path | Method | Description |
|------|--------|-------------|
| `/health` | GET | Returns `{"status":"ok"}` — no auth required; for load balancer health checks |
| `/auth/start` | GET | Initiates OAuth PKCE flow; redirects to provider |
| `/auth/callback` | GET | OAuth callback; exchanges code for token |

All other paths are subject to the full security pipeline: auth → rate limit → RBAC → proxy → audit.

---

## Error responses

The proxy returns JSON-RPC 2.0 error responses for security denials:

| HTTP Status | JSON-RPC Code | Reason |
|-------------|---------------|--------|
| 401 | `-32001` | Missing or invalid Bearer token |
| 403 | `-32002` | RBAC policy denied the request |
| 429 | `-32003` | Per-client rate limit exceeded |

Example 401 response:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32001,
    "message": "Unauthorized: missing Authorization header"
  },
  "id": 1
}
```
