# Configuration Reference

Complete reference for `config.yaml`. All fields shown with types, defaults, and examples.

---

## Complete example — single server

```yaml
server:
  upstream_url: "http://localhost:3000"
  listen_addr: ":8080"

auth:
  provider: "github"
  client_id: "your-client-id"
  client_secret: "${OAUTH_CLIENT_SECRET}"
  redirect_url: "http://localhost:8080/auth/callback"
```

## Complete example — multiple servers

```yaml
server:
  listen_addr: ":8080"
  registry:
    default: "files"  # fallback for unmatched paths
    servers:
      - name: "files"
        url: "http://files-server:3000"
        enabled: true
      - name: "database"
        url: "http://database-server:5432"
        enabled: true
        timeout: 60
      - name: "github"
        url: "http://github-mcp:8080"
        enabled: true
      - name: "postgres"
        url: "http://postgres-mcp:5432"
        enabled: false  # disabled but configured

auth:
  provider: "github"
  client_id: "your-client-id"
  client_secret: "${OAUTH_CLIENT_SECRET}"
  redirect_url: "http://localhost:8080/auth/callback"
```

user_roles:
  default: "readonly"
  mapping:
    "alice@company.com": "admin"
  claim_mapping:
    - claim: "groups"
      operator: "contains"
      value: "administrators"
      role: "admin"

roles:
  # Built-in roles are always present — you can override their permissions
  - name: "admin"
    allowed_tools: []
    deny_tools: []
  - name: "readonly"
    allowed_tools: []
    deny_tools: []
  - name: "restricted"
    allowed_tools: ["read_file", "search"]
    deny_tools: []
  
  # Custom roles: extend beyond the built-in three
  - name: "devops"
    allowed_tools:
      - "tools/list"
      - "tools/call"
      - "resources/read"
      - "resources/list"
    deny_tools:
      - "delete_resource"
      
  - name: "analyst"
    allowed_tools:
      - "tools/list"
      - "resources/read"
      - "prompts/list"
      - "prompts/get"
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
- Rate-limits to 60 req/min with burst of 10 (free tier defaults)
- Writes audit logs to stdout
- Logs at INFO level in JSON format

---

## Environment variable substitution

Any string value in the config supports `${ENV_VAR}` substitution. The substitution happens before YAML parsing:

```yaml
auth:
  client_secret: "${OAUTH_CLIENT_SECRET}"  # reads from environment at startup
```

If the variable is unset, the literal string `${ENV_VAR}` is used (no error). **The proxy logs a warning at startup listing all unresolved variable names**, helping you catch configuration mistakes early.

```bash
 WARN Unresolved environment variable references in config; values left as literal strings
       vars=[DB_PASSWORD,REDIS_URL]
       config=/etc/mcpproxy/config.yaml
```

Use this for all secrets — never hardcode credentials in the config file.

---

## server

Controls where the proxy listens and how it routes requests to upstream MCP servers.

### Single-server vs. multi-server routing

The proxy supports two modes:

**Single-server mode** (simple, backward compatible):
- Use `server.upstream_url` to point to one MCP server
- All requests go to that server

**Multi-server mode** (recommended for scaling):
- Use `server.registry.servers` to define multiple MCP servers
- Requests are routed by path prefix (`/files/...` → files server, `/database/...` → database server)
- More flexible, supports server discovery and independent configuration

---

### server.upstream_url

  | | |
  |---|---|
  | **Type** | string |
  | **Required** | yes (if not `server.registry`) |
  | **Default** | none |

The URL of the upstream MCP server for single-server mode. All authenticated, authorized requests are forwarded here. Must include scheme and host.

**Note:** This field is deprecated in favor of `server.registry`. Kept for backward compatibility. If both are set, `server.registry` takes precedence.

```yaml
server:
  upstream_url: "http://localhost:3000"
  # upstream_url: "http://mcp-server.internal:8000"
  # upstream_url: "https://mcp-server.example.com"
```

The proxy preserves the request path when forwarding. A request to `http://proxy:8080/` is forwarded to `http://upstream:3000/`.

---

### server.registry

| | |
|---|---|
| **Type** | ServerRegistryConfig |
| **Required** | yes (if not `upstream_url`) |
| **Default** | none |

Multi-server routing configuration. When present, the proxy routes requests to different upstream MCP servers based on the first path segment.

**Routing behavior:**
- Request path `/files/tools/list` → routes to server named `files`
- Request path `/database/query` → routes to server named `database`
- Request path `/tools/list` (no server prefix) → routes to `default` server, or returns 404

**Tools aggregation:** When clients call `/tools/list`, they see tools from all servers with namespaced names:
- `"files.read_file"`, `"files.list_directory"`
- `"database.query"`, `"database.insert"`
- `"github.create_issue"`, `"github.search"`

```yaml
server:
  registry:
    default: "files"  # fallback server for unmatched paths
    servers:
      - name: "files"
        url: "http://files-server:3000"
        enabled: true
      - name: "database"
        url: "http://database-server:5432"
        enabled: true
        timeout: 60
      - name: "github"
        url: "http://github-mcp:8080"
        enabled: true
```

#### server.registry.default

| | |
|---|---|
| **Type** | string |
| **Required** | no |
| **Default** | none (returns 404 for unmatched paths) |

The fallback server name for requests without a path prefix (e.g., `/tools/list` instead of `/files/tools/list`). Must match one of the `server.registry.servers[].name` values.

**Example:**
```yaml
server:
  registry:
    default: "files"  # Routes /tools/list to files server
    servers:
      - name: "files"
        url: "http://files-server:3000"
```

**Behavior:**
- `POST /files/tools/list` → files server
- `POST /tools/list` → files server (default)
- `POST /nonexistent/tools/list` → 404 (server not found)

If omitted, requests without a server prefix return 404.

#### server.registry.servers

| | |
|---|-|
| **Type** | []UpstreamServerConfig |
| **Required** | yes (when using `registry`) |
| **Default** | none |

List of upstream MCP server configurations. Each server has a unique `name` used for path routing.

**Example:**
```yaml
server:
  registry:
    servers:
      - name: "files"
        url: "http://files-server:3000"
      - name: "database"
        url: "http://database-server:5432"
        timeout: 60
      - name: "github"
        url: "http://github-mcp:8080"
      - name: "legacy"
        url: "http://legacy-server:9000"
        enabled: false  # server is configured but disabled
```

##### UpstreamServerConfig fields

| Field | Type | Required | Default | Description |
|---|-|-|-|---|
| `name` | string | yes | — | Server identifier used in path routing (e.g., `/files/...`) |
| `url` | string | yes | — | Upstream server URL (e.g., `http://localhost:3000`) |
| `enabled` | boolean | no | `true` | Whether this server accepts requests |
| `timeout` | integer | no | `120` | HTTP timeout in seconds for this server |
| `tags` | []string | no | — | Arbitrary labels for server grouping |

**Constraints:**
- `name` must be unique across all servers
- `name` must be URL-safe (alphanumeric, hyphens, underscores)
- `url` must include scheme (`http://` or `https://`)
- Disabled servers (`enabled: false`) return 404 on all requests

**Example with all fields:**
```yaml
server:
  registry:
    servers:
      - name: "files"
        url: "http://files-server:3000"
        enabled: true
        timeout: 120
        tags: ["internal", "primary"]
```

---

### server.listen_addr

| | |
|---|---|
| **Type** | string |
| **Required** | no |
| **Default** | `":8080"` |

The TCP address the proxy HTTP server listens on. Standard Go net/http address format: `[host]:port`.

```yaml
server:
  listen_addr: ":9000"            # custom port
```

---

### server.tls

TLS configuration for the proxy server.

| Field | Type | Default | Description |
|------|-|---|---|
| `cert_file` | string | none | Path to TLS certificate file (PEM format) |
| `key_file` | string | none | Path to TLS private key file (PEM format) |
| `min_version` | string | "1.2" | Minimum TLS version: "1.2" or "1.3" |
| `cipher_suites` | []string | Go defaults | Optional list of cipher suite names |
| `required` | bool | false | If true, proxy refuses to start without valid TLS |

**Example — basic TLS:**
```yaml
server:
  tls:
    cert_file: "/etc/ssl/certs/proxy.crt"
    key_file: "/etc/ssl/private/proxy.key"
    min_version: "1.2"
```

**Example — enforce TLS (refuse to start without it):**
```yaml
server:
  tls:
    cert_file: "/etc/ssl/certs/proxy.crt"
    key_file: "/etc/ssl/private/proxy.key"
    required: true
    min_version: "1.3"
    cipher_suites:
      - "TLS_AES_256_GCM_SHA384"
      - "TLS_CHACHA20_POLY1305_SHA256"
```

**Note:** TLS 1.0 and 1.1 are disabled for security. Only "1.2" and "1.3" are valid values for `min_version`.

**Behavior when `required: true`:** If `required` is set to `true` but neither `cert_file` nor `key_file` are configured, the proxy exits with a fatal error:

```
 FATAL TLS is required by configuration but not configured: cert_file and key_file must be set
```

When TLS is not enabled, a warning is logged:

```
 WARN TLS disabled (plain HTTP) — auth tokens and sessions will travel unencrypted
```

---

## user_roles

Maps authenticated users to RBAC roles via email matching or OAuth claim evaluation.

**Role resolution order:**
1. **Claim mapping** (`claim_mapping`) — evaluated first, first match wins
2. **Email mapping** (`mapping`) — evaluated if no claim rule matches
3. **Default role** (`default`) — fallback when neither claim nor email matches

### user_roles.mapping

| | |
|---|---|
| **Type** | map[string]string |
| **Required** | no |
| **Default** | `{}` |

Maps user email addresses to role names. Supports both built-in roles (`admin`, `readonly`, `restricted`) and custom roles you define in `roles`.

```yaml
user_roles:
  mapping:
    "alice@company.com": "admin"
    "bob@company.com": "readonly"
    "dev-team@company.com": "devops"  # custom role
```

### user_roles.default

| | |
|---|---|
| **Type** | string |
| **Required** | no |
| **Default** | `"readonly"` |

The role assigned to authenticated users who don't match any email in `mapping` and don't match any claim rule in `claim_mapping`. Can be a built-in role or a custom role.

```yaml
user_roles:
  default: "readonly"  # (default)
  # default: "devops"  # custom role as default
```

### user_roles.claim_mapping

| | |
|---|---|
| **Type** | []ClaimRule |
| **Required** | no |
| **Default** | `[]` |

A list of rules to automatically assign roles based on OAuth claims. This is ideal for enterprise deployments where you want roles to be managed in your identity provider rather than hardcoded in the config.

**How it works:**
1. Each rule specifies an OAuth claim name, an operator, a value, and a role
2. When a user authenticates, their claims are checked against each rule in order
3. The first matching rule determines the user's role
4. If no claim rule matches, falls back to email mapping (`mapping`) or default (`default`)

**Enterprise example with claims and custom roles:**

```yaml
user_roles:
  default: "readonly"
  claim_mapping:
    # Assign "admin" role to users in the "administrators" group
    - claim: "groups"
      operator: "contains"
      value: "administrators"
      role: "admin"
    
    # Assign "devops" custom role to Engineering team with SRE access
    - claim: "department"
      operator: "equals"
      value: "engineering"
      role: "devops"
    
    # Assign "analyst" custom role to Business Intelligence team
    - claim: "department"
      operator: "equals"
      value: "business-intelligence"
      role: "analyst"
    
    # Match security level using regex for restricted access
    - claim: "security_clearance"
      operator: "regex"
      value: "^L[12].*"
      role: "restricted"
```

**Available operators:**

| Operator | Description | Example |
|---|---|---|
| `equals` | Exact match | `department: "engineering"` matches `"engineering"` |
| `contains` | Check if claim contains the value | `groups: "administrators"` matches `"sales,administrators,auditors"` |
| `starts_with` | Check if claim starts with the value | `role: "admin"` matches `"admin_user"` |
| `ends_with` | Check if claim ends with the value | `role: "_admin"` matches `"super_admin"` |
| `regex` | Match claim against a regular expression | `security_clearance: "^L[34].*"` matches `"L3_CONFIDENTIAL"` |

**Common OAuth claims to use:**

- **GitHub:** `email`, `login`, `organization` (from token scopes like `read:org`)
- **Google:** `email`, `name`, `groups`, `domain`
- **Okta:** `email`, `groups`, `department`, `manager`, custom claims from profiles
- **Auth0:** `email`, `name`, `roles`, `permissions`, custom claims from rules
- **Azure AD:** `email`, `oids`, `roles`, `groups`, `department`

**Note:** The claim names depend on what your OAuth provider includes in the token. Check your provider's documentation for available claims. You may need to configure custom claims or token scopes to get the data you need.

---

## roles

Defines the RBAC permissions for each role. Three roles are always available: `admin`, `readonly`, `restricted`. If this section is omitted, default permissions are applied.

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

### auth.max_token_cache_size

| | |
|---|---|
| **Type** | integer |
| **Required** | no |
| **Default** | `1000` |

Maximum number of validated tokens to cache in memory. Uses LRU (least recently used) eviction when the cache exceeds this limit. Higher values reduce OAuth provider API calls but consume more memory.

**Why it matters:** Each authenticated request validates the token with the OAuth provider. The cache stores validated tokens for 5 minutes, avoiding repeated validation calls. In high-traffic scenarios, a larger cache can significantly reduce latency and OAuth provider API usage.

```yaml
auth:
  max_token_cache_size: 1000   # (default)
  # max_token_cache_size: 5000  # for high-traffic production
  # max_token_cache_size: 500   # for memory-constrained environments
```

### auth.session

Session storage configuration. Controls how long sessions last and whether they persist across proxy restarts.

| Field | Type | Default | Description |
|------|-|---|-|
| `ttl` | string | "24h" | Session validity duration (Go duration format) |
| `backend` | string | "memory" | Storage backend: "memory" or "file" |
| `filepath` | string | "./sessions.json" | Path to session file (when backend="file") |

**Example — default in-memory sessions (expire on restart):**
```yaml
auth:
  session:
    ttl: "24h"          # (default)
    backend: "memory"    # (default)
```

**Example — persistent sessions on disk:**
```yaml
auth:
  session:
    ttl: "168h"         # 7 days
    backend: "file"
    filepath: "/var/lib/mcpproxy/sessions.json"
```

**Backend options:**
- **`memory`** — Sessions stored in RAM. Fast but lost on proxy restart. Best for development or when users can re-authenticate easily.
- **`file`** — Sessions persisted to disk as JSON. Survives proxy restarts. Users stay logged in across deployments. Suitable for production with persistent storage.

**TTL format:** Uses Go duration strings: `"1h"`, `"30m"`, `"168h"` (7 days), `"168000000000"` (168 hours in nanoseconds).

**Session refresh:** Sessions are automatically refreshed on each successful authentication, extending the TTL from the last access time.

---

## roles

Defines the RBAC permissions for each role. Three built-in roles (`admin`, `readonly`, `restricted`) are always present. You can override their permissions or add unlimited custom roles for fine-grained access control.

### Built-in vs. Custom Roles

The proxy always provides three built-in roles:
- **admin**: Full access to all MCP methods and tools
- **readonly**: Can list and read resources, but cannot call tools
- **restricted**: Limited to explicitly allowed tools

You can extend these with custom roles to model your organization's access patterns.

### Role definitions

Each entry in the `roles` list has:

| Field | Type | Description |
|------|------|------------|
| `name` | string | Role name. Can be a built-in role name (to override) or any custom name |
| `allowed_tools` | []string | Tool names this role may call via `tools/call`. Empty list = all tools allowed (admin). |
| `deny_tools` | []string | Tool names explicitly denied. Deny rules take precedence over allow rules. |

### How role merging works

When you define roles in the config:
- Built-in roles with your custom permissions override the defaults
- Custom roles are added alongside the built-in three
- You can reference any defined role in `user_roles.mapping`, `user_roles.default`, or `user_roles.claim_mapping`

**Example:** If you only define a `devops` role, the final configuration includes four roles: `admin`, `readonly`, `restricted` (with default permissions) + `devops` (with your custom permissions).

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

**Custom roles:**
- Behave like `restricted` — you must explicitly define `allowed_tools`
- Can use any tool names available in your MCP server
- Can define `deny_tools` to block specific tools even if they match `allowed_tools`

### Example role configurations

**Basic — override built-in roles only:**

```yaml
roles:
  - name: "admin"
    allowed_tools: []
    deny_tools:
      - "delete_database"  # Admin cannot delete databases

  - name: "readonly"
    allowed_tools: []
    deny_tools: []

  - name: "restricted"
    allowed_tools:
      - "read_file"
      - "search_files"
    deny_tools: []
```

**Extended — add custom roles for teams:**

```yaml
roles:
  # Override admin to block dangerous operations
  - name: "admin"
    allowed_tools: []
    deny_tools:
      - "delete_table"
      - "drop_database"

  # DevOps team: full tool access minus destructive operations
  - name: "devops"
    allowed_tools:
      - "tools/list"
      - "tools/call"
      - "resources/read"
      - "resources/list"
      - "prompts/list"
    deny_tools:
      - "delete_resource"
      - "delete_file"

  # Data analysts: read-only access to data tools
  - name: "analyst"
    allowed_tools:
      - "read_file"
      - "search_files"
      - "query_database"
      - "generate_report"
    deny_tools: []

  # Interns: extremely limited access
  - name: "intern"
    allowed_tools:
      - "read_file"
      - "search_files"
    deny_tools:
      - "read_password_file"
```

**Denying specific tools:**

```yaml
roles:
  - name: "admin"
    allowed_tools: []
    deny_tools:
      - "delete_table"
      - "drop_database"
```

`deny_tools` takes precedence over `allowed_tools`. An admin with `deny_tools: ["dangerous_tool"]` cannot call `dangerous_tool` even though `allowed_tools` is empty (all allowed).

### Using custom roles with claim mapping

**Complete enterprise example:**

```yaml
# Define custom roles for your organization
roles:
  - name: "admin"
    allowed_tools: []
    deny_tools:
      - "delete_database"

  - name: "devops"
    allowed_tools:
      - "tools/list"
      - "tools/call"
      - "resources/read"
    deny_tools:
      - "delete_resource"

  - name: "analyst"
    allowed_tools:
      - "read_file"
      - "query_database"
      - "generate_report"
    deny_tools: []

  - name: "intern"
    allowed_tools:
      - "read_file"
      - "search_files"
    deny_tools: []

# Assign roles automatically based on OAuth claims
user_roles:
  default: "readonly"
  claim_mapping:
    # Users in "administrators" group get admin role
    - claim: "groups"
      operator: "contains"
      value: "administrators"
      role: "admin"
    
    # Engineering department gets devops role
    - claim: "department"
      operator: "equals"
      value: "engineering"
      role: "devops"
    
    # Business Intelligence gets analyst role  
    - claim: "department"
      operator: "equals"
      value: "business-intelligence"
      role: "analyst"
    
    # Interns identified by email prefix
    - claim: "email"
      operator: "starts_with"
      value: "intern."
      role: "intern"

# Explicit email mappings override claim-based assignment
user_roles:
  mapping:
    "alice@company.com": "admin"  # Alice is always admin regardless of claims
    "bob@company.com": "devops"
```

---

## rate_limit

Per-client request rate limiting using a token bucket algorithm. Limits apply per authenticated client ID.

### rate_limit.requests_per_minute

| | |
|---|---|
| **Type** | integer |
| **Required** | no |
| **Default** | `60` |

Sustained request rate per client, in requests per minute. The token bucket refills at this rate. The default of 60 req/min matches the free tier limits, but you can raise this for your internal deployments.

```yaml
rate_limit:
  requests_per_minute: 60    # 1 req/second (free tier default)
  # requests_per_minute: 300 # 5 req/second (internal use)
  # requests_per_minute: 600 # 10 req/second
```

### rate_limit.burst_size

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
  burst_size: 10   # can send 10 requests instantly, then 60/min sustained (default)
  # burst_size: 50  # larger burst for clients with bursty patterns
```

---

## audit

Controls the immutable audit trail. Every request (allowed and denied) is logged as a JSONL record. Supports both local output (stdout/file) and external sinks (Azure Sentinel, SIEMs, custom webhooks).

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
| **Values** | `"stdout"`, `"file"`, `"both"`, `"off"` |

Where audit log entries are written.

- `"stdout"` — standard output; works well with Docker and log aggregators (Splunk, Datadog, CloudWatch)
- `"file"` — writes to `audit.file_path`; requires `file_path` to be set
- `"both"` — writes to both stdout and file
- `"off"` — disable stdout/file output (use external sinks only)

```yaml
audit:
  output: "stdout"  # (default)
  # output: "file"
  # output: "both"
  # output: "off"  # Use external sinks only
```

### audit.file_path

| | |
|---|---|
| **Type** | string |
| **Required** | yes when `output: "file"` or `output: "both"` |
| **Default** | `""` |

Path to the audit log file.

```yaml
audit:
  output: "file"
  file_path: "/var/log/mcpproxy/audit.jsonl"
```

### audit.rotation

Controls audit log file rotation for file output.

| Field | Type | Default | Description |
|-|-|-|---|
| `max_size_mb` | integer | 0 (disabled) | Rotate file when it reaches this size in MB |
| `max_age_hours` | integer | 0 (disabled) | Delete rotated files older than this many hours |
| `max_backups` | integer | 1 | Maximum number of rotated files to retain |

**Rotation behavior:** When rotation occurs, existing backup files are cascaded to make room:
- `.1` → `.2` → `.3` → ... → `.max_backups`
- The oldest file (`.max_backups`) is deleted if it exists
- The current file becomes `.1`
- A new empty file is created for continued logging

**Example — retain 5 rotated files:**
```yaml
audit:
  rotation:
    max_size_mb: 100     # Rotate when file reaches 100MB
    max_age_hours: 168   # Delete files older than 168 hours (7 days)
    max_backups: 5       # Keep audit.jsonl.1 through audit.jsonl.5
```

**Resulting files after multiple rotations with `max_backups: 5`:**
```
audit.jsonl        # Current log file
audit.jsonl.1      # Most recent rotation
audit.jsonl.2      
audit.jsonl.3
audit.jsonl.4
audit.jsonl.5      # Oldest retained rotation (deleted on next rotation)
```

**Note:** `max_backups` defaults to 1 for backward compatibility, retaining only the single most recent rotated file.

---

## audit.sinks (Pluggable Audit Sinks)

The audit framework supports multiple external sinks that fan out events asynchronously. Each sink operates independently and supports event filtering.

**Available sink types:**
- **`ocsf`** — Open Cybersecurity Schema Framework (OCFS 12.0.0) for Azure Sentinel and OCSF-compatible SIEMs
- **`cef`** — Common Event Format for syslog/SIEM systems (supports UDP, TCP, TCP-TLS, HTTPS)
- **`json_http`** — Raw JSON over HTTPS for custom webhooks and endpoints

### Sink Configuration Structure

Each sink has the following structure:

```yaml
sinks:
  - type: {sink_type}
    enabled: true        # Whether this sink is active (default: true)
    name: "{name}"       # Optional: human-readable name for the sink
    {sink_type}:         # Sink-specific configuration
      ...
    filter:              # Optional: event filtering criteria
      methods: [...]
      tools: [...]
      results: [...]
      client_ids: [...]
```

### Event Filtering

All sinks support filtering via the `filter` field:

| Field | Type | Description |
|---|---|---|
| `methods` | []string | MCP methods to include (e.g., `["tools/call"]`). Empty = all methods |
| `tools` | []string | Tool names to include (supports `*` wildcards). Empty = all tools |
| `results` | []string | Outcomes: `["allowed"]`, `["denied"]`, or both. Empty = both |
| `client_ids` | []string | Client IDs to include. Empty = all clients |

**Filter combinations are AND — all conditions must match.**

---

### OCSF Sink (Azure Sentinel)

Sends audit events transformed to [OCSF 12.0.0](https://schema-registry.osso Standard) format to Azure Log Analytics via the Logs Ingestion API.

**Use case:** Enterprise security monitoring with Azure Sentinel, Microsoft Defender for Cloud, or other OCSF-compatible platforms.

| Field | Type | Default | Description |
|-|-|-|---|
| `workspace_id` | string | required | Azure Log Analytics workspace ID (32-character hex string) |
| `api_key` | string | required | Workspace primary key (use `${ENV_VAR}` for security) |
| `batch_size` | integer | 100 | Maximum events per batch before sending |
| `flush_interval` | integer | 5 | Seconds to wait before flushing pending batch |
| `timeout` | integer | 30 | HTTP request timeout in seconds |
| `buffer_size` | integer | 1000 | Maximum events to buffer before dropping |

**Example:**
```yaml
audit:
  sinks:
    - type: ocsf
      enabled: true
      name: "azure-sentinel"
      ocsf:
        workspace_id: "{YOUR_WORKSPACE_ID}"
        api_key: "${AZURE_SENTINEL_API_KEY}"
        batch_size: 100
        flush_interval: 5
        timeout: 30
        buffer_size: 1000
      
      filter:
        methods: ["tools/call"]
        results: ["denied"]
```

**Events transformed to OCSF fields:**
- `event_category`: `"access"`
- `event_subtype`: `"access_success"` or `"access_denied"`
- `result`: `"success"` or `"denied"`
- `device.id`: MCP proxy identifier
- `user.id`: Client ID
- `target.name`: MCP method name
- `detail.latency_ms`: Request latency

---

### CEF Sink (Syslog/SIEM)

Sends audit events in [Common Event Format](https://docs.logr.io/logr/cef/) to syslog servers, SIEMs, or log collectors.

**Use case:** Integration with enterprise SIEMs (Splunk, QRadar, ArcSight, LogRhythm) or syslog infrastructure.

#### Transport Options

| Transport | Description | Default Port |
|---|---|---|
| `udp` | UDP syslog (fire-and-forget, may drop packets) | 514 |
| `tcp` | Reliable TCP delivery | 514 |
| `tcp_tls` | Encrypted TCP with TLS (recommended for production) | 6514 |
| `https` | REST API over HTTPS (batched delivery) | 443 |

| Field | Type | Default | Description |
|-|-|-|---|
| `transport` | string | `"udp"` | Transport protocol (`udp`, `tcp`, `tcp_tls`, `https`) |
| `host` | string | required | Syslog server hostname or IP address |
| `port` | integer | transport-specific | Port number (514 for UDP/TCP, 6514 for TCP-TLS, 443 for HTTPS) |
| `facility` | string | `"local0"` | Syslog facility (`local0`–`local7`, `daemon`, `auth`, etc.) |
| `timeout` | integer | 5 | Connection timeout in seconds |
| `buffer_size` | integer | 1000 | Maximum events to buffer before dropping |
| `batch_size` | integer | 10 | Batch size (HTTPS only) |
| `flush_interval` | integer | 1 | Flush interval in seconds (HTTPS only) |

**Example — TCP-TLS to SIEM:**
```yaml
audit:
  sinks:
    - type: cef
      enabled: true
      name: "enterprise-siem"
      cef:
        transport: "tcp_tls"
        host: "siem.company.com"
        port: 6514
        facility: "local0"
        timeout: 5
        buffer_size: 2000
      
      filter:
        methods: ["tools/call", "resources/read"]
        tools: ["*data*", "*database*"]
```

**Example — HTTPS batched delivery:**
```yaml
audit:
  sinks:
    - type: cef
      enabled: true
      name: "rest-siem"
      cef:
        transport: "https"
        host: "logs.company.com"
        port: 443
        batch_size: 50
        flush_interval: 2
```

**CEF format:**
```
CEF:0|MCPZeroTrust|Proxy|1.0||AUTH:Authorization|AUTH::AccessDenied|8|src=0.0.0.0 dstPort=8080 user=user123 action=deny method=tools/call tool=read_secrets request_id=req-xyz reason=rbac-tool-not-allowed
```

---

### JSON-HTTP Sink (Custom Webhook)

Sends raw JSON audit events to a custom HTTPS endpoint.

**Use case:** Custom log aggregators, internal audit systems, webhook-based integrations, or forwarding to other services.

| Field | Type | Default | Description |
|-|-|-|---|
| `endpoint` | string | required | HTTPS URL to send events to |
| `headers` | map[string]string | `{}` | Custom HTTP headers to include |
| `batch_size` | integer | 50 | Maximum events per batch |
| `flush_interval` | integer | 10 | Seconds to wait before flushing batch |
| `timeout` | integer | 15 | HTTP request timeout in seconds |
| `buffer_size` | integer | 1000 | Maximum events to buffer before dropping |
| `basic_auth.username` | string | `""` | Basic auth username (optional) |
| `basic_auth.password` | string | `""` | Basic auth password (use `${ENV_VAR}`) |
| `bearer_token` | string | `""` | Bearer token for auth (use `${ENV_VAR}`) |

**Example — Basic Auth webhook:**
```yaml
audit:
  sinks:
    - type: json_http
      enabled: true
      name: "internal-audit-system"
      json_http:
        endpoint: "https://audit.internal.company.com/api/v1/events"
        headers:
          X-Tenant-ID: "tenant-123"
          X-Environment: "production"
        batch_size: 50
        flush_interval: 10
        timeout: 15
        buffer_size: 1000
        basic_auth:
          username: "audit-service"
          password: "${AUDIT_WEBHOOK_PASSWORD}"
      
      filter:
        methods: ["tools/call"]
        results: ["denied"]
```

**Example — Bearer token:**
```yaml
audit:
  sinks:
    - type: json_http
      enabled: true
      name: "datadog-webhook"
      json_http:
        endpoint: "https://http-intake.logs.datad0g.com/api/v2/logs"
        headers:
          DD-Source: "mcp-proxy"
          DD-Service: "mcp-zero-trust"
        bearer_token: "${DATADOG_API_KEY}"
        batch_size: 100
        flush_interval: 5
```

---

### Complete Example — Multiple Sinks with Filtering

```yaml
audit:
  enabled: true
  output: "both"
  file_path: "/var/log/mcpproxy/audit.jsonl"
  rotation:
    max_size_mb: 100
    max_age_hours: 168
  
  sinks:
    # Send all denied events to Azure Sentinel
    - type: ocsf
      enabled: true
      name: "azure-sentinel"
      ocsf:
        workspace_id: "{WORKSPACE_ID}"
        api_key: "${AZURE_API_KEY}"
        batch_size: 100
        flush_interval: 5
      filter:
        results: ["denied"]
    
    # Send all tool calls to SIEM for compliance monitoring
    - type: cef
      enabled: true
      name: "siem-compliance"
      cef:
        transport: "tcp_tls"
        host: "siem.company.com"
        port: 6514
        facility: "local0"
      filter:
        methods: ["tools/call"]
        tools: ["*data*", "*database*", "*secret*"]
    
    # Send admin actions to internal audit system
    - type: json_http
      enabled: true
      name: "internal-audit"
      json_http:
        endpoint: "https://audit.internal/api/events"
        bearer_token: "${AUDIT_API_KEY}"
        batch_size: 50
        flush_interval: 10
      filter:
        client_ids: ["admin@company.com", "security@company.com"]
```

**Notes:**
- All sinks run asynchronously — sink failures don't block request processing
- Events may be dropped if buffer is full (configurable via `buffer_size`)
- Each sink independently applies its filter — the same event can go to multiple sinks
- Use `${ENV_VAR}` for all secrets (API keys, passwords, tokens)

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
|------|--|------|
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
