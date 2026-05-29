# MCP Zero-Trust Proxy

A drop-in reverse proxy that adds zero-trust security to any [MCP](https://modelcontextprotocol.io) server — no code changes required.

```bash
docker run -e MCP_TARGET=localhost:3000 -e AUTH_PROVIDER=github -p 8080:8080 \
  ghcr.io/anoblescm/mcp-zero-trust-proxy
```

## What it does

MCP (Model Context Protocol) is how AI agents connect to tools — Claude, Cursor, Copilot all use it. But authentication is optional in the spec. This proxy sits between your MCP clients and servers to enforce security:

- **Multi-server routing** — Configure multiple MCP servers with path-based routing (`/files/...` → files server, `/db/...` → database server)
- **OAuth 2.1 PKCE** — Require login via GitHub, Google, Microsoft Entra ID, Okta, or any OIDC provider
- **Tool-level RBAC** — Control which tools each user can call with built-in roles (admin/readonly/restricted) and unlimited custom roles
- **Claim-based role mapping** — Automatic role assignment from OAuth claims (e.g., assign "devops" role to Engineering department)
- **Per-client sessions** — Each user gets their own session boundary
- **Pluggable audit sinks** — Forward events to Azure Sentinel (OCSF), SIEMs (CEF/syslog), or custom webhooks (JSON over HTTPS)
- **Rate limiting** — Per-client token bucket (default: 60 req/min free tier, configurable)

## Why

- 8,000+ MCP servers publicly reachable with no authentication ([Shodan](https://www.shodan.io/search?query=mcp))
- 30+ CVEs in 60 days (Jan-Feb 2026)
- CVSS 9.6 RCE in `mcp-remote`, the most popular OAuth workaround
- Clawdbot breach hit 1,800+ servers in 48 hours

## Quick start

### Single server (simple)

```bash
# 1. Pull
docker pull ghcr.io/anoblescm/mcp-zero-trust-proxy:latest

# 2. Create config.yaml
cat > config.yaml <<EOF
server:
  upstream_url: "http://host.docker.internal:3000"
  listen_addr: ":8080"
auth:
  provider: "github"
  client_id: "your-github-client-id"
  client_secret: "${OAUTH_CLIENT_SECRET}"
  redirect_url: "http://localhost:8080/auth/callback"
roles:
  - name: "admin"
    allowed_tools: []
  - name: "readonly"
    allowed_tools: []
audit:
  enabled: true
  output: "stdout"
EOF

# 3. Run
docker run -p 8080:8080 \
  -v ./config.yaml:/etc/mcpproxy/config.yaml \
  -e OAUTH_CLIENT_SECRET=your_secret \
  ghcr.io/anoblescm/mcp-zero-trust-proxy:latest \
  --config /etc/mcpproxy/config.yaml

# 4. Verify
curl http://localhost:8080/health
# {"status":"ok"}
```

### Multiple servers (path-based routing)

Route requests to different MCP servers based on the path prefix:

```yaml
server:
  listen_addr: ":8080"
  registry:
    default: "files"  # fallback server for unmatched paths
    servers:
      - name: "files"
        url: "http://files-server:3000"
        enabled: true
        timeout: 120
      - name: "database"
        url: "http://db-server:5432"
        enabled: true
        timeout: 60
      - name: "github"
        url: "http://github-mcp:8080"
        enabled: true
      - name: "postgres"
        url: "http://postgres-mcp:5432"
        enabled: false  # disabled
```

**How routing works:**
- `POST /files/tools/list` → files server at `http://files-server:3000/tools/list`
- `POST /database/tools/call` → database server at `http://db-server:5432/tools/call`
- `POST /tools/list` → default server (files) at `http://files-server:3000/tools/list`
- Requests to disabled servers return 404

**Benefits of multi-server routing:**
- **Unified access**: One proxy endpoint for multiple MCP servers
- **Server discovery**: Clients see all tools with namespaced names (`files.read_file`, `database.query`)
- **Independent configuration**: Each server can have different timeouts, can be enabled/disabled independently
- **Fallback routing**: Default server handles unmatched paths

### Build from source

```bash
# Build for native platform
docker build -t mcpzerotrust/proxy:latest .

# Build multi-arch image (requires buildx)
docker buildx build --platform linux/amd64,linux/arm64 \
  -t mcpzerotrust/proxy:latest --push .

# Run with docker-compose
docker compose up -d
```

**Image sizes:**
- Runtime image: ~17MB (alpine:3.19 + static binary)
- Builder image: ~120MB (golang:1.24-alpine)

**Health check:**

The proxy includes a built-in health endpoint for container orchestration:

```bash
# Check health
curl http://localhost:8080/health
# {"status":"ok"}

# Docker reports health status
docker inspect --format='{{.State.Health.Status}}' <container_id>
# healthy
```

Docker HEALTHCHECK configuration:
- Interval: 30s
- Timeout: 5s
- Start period: 5s
- Retries: 3

### Binary

```bash
go install github.com/keith-aykira/mcp-zero-trust-proxy/cmd/mcpproxy@latest
mcpproxy --config ./config.yaml
```

Or download a release binary from the [releases page](https://github.com/keith-aykira/mcp-zero-trust-proxy/releases).

## How it works (single server)

```
AI Client (Claude, Cursor, Copilot)
        │
        ▼
┌──────────────────────┐
│ MCP Zero-Trust Proxy │
│                      │
│  1. Body size check  │
│  2. Auth (OAuth 2.1) │
│  3. Rate limit       │
│  4. JSON-RPC parse   │
│  5. RBAC check       │
│  6. Forward to UPSTREAM SERVER │
│  7. Filter tools/list│
│  8. Audit log        │
└─────━━━━━━━━───────┘
        │
        ▼
   Your MCP Server
   (unchanged)
```

## How it works

```
AI Client (Claude, Cursor, Copilot)
        │
        ▼  
┌───━━━━━━━●───────┐
│  MCP Zero-Trust Proxy │
│                   │
│   ● Path router:  │
│     /files/* → Files Server     │
│     /database/* → Database Server │ 
│     /github/* → GitHub MCP      │
│     /* → Default server (or 404) │
└───┬─────┬─────┬────┘
    │     │     │
    ▼     ▼     ▼
┌─────┐ ┌──────┐ ┌──────┐
│Files│ │Database│ │GitHub│
│Server│ │ Server │ │ MCP  │
└─────┘ └──────┘ └──────┘
```

**Request flow with multi-server:**
1. Request arrives at proxy (e.g., `POST /database/tools/call`)
2. Path router extracts server name from first path segment (`database`)
3. Routes to configured upstream (`http://db-server:5432/tools/call`)
4. Standard security pipeline runs: auth → rate limit → RBAC → proxy → audit
5. Response returned to client with tools namespaced (`database.query`, `files.read_file`)

## Configuration

All config is in a single YAML file. See [`configs/example.yaml`](configs/example.yaml) for the full reference.

**RBAC with custom roles:**

```yaml
roles:
  # Admin: full access except database deletion
  - name: "admin"
    allowed_tools: []
    deny_tools:
      - "delete_database"

  # Readonly: view only
  - name: "readonly"
    allowed_tools:
      - "tools/list"
      - "resources/list"
      - "resources/read"

  # DevOps: full tool access except destructive operations
  - name: "devops"
    allowed_tools:
      - "tools/list"
      - "tools/call"
      - "resources/read"
    deny_tools:
      - "delete_resource"
      - "delete_file"

user_roles:
  default: "readonly"
  mapping:
    "alice@company.com": "admin"
    "bob@company.com": "devops"
  claim_mapping:
    # Engineering team gets devops role
    - claim: "department"
      operator: "equals"
      value: "engineering"
      role: "devops"
    # Admins group gets admin role
    - claim: "groups"
      operator: "contains"
      value: "administrators"
      role: "admin"
```

**Audit logging with external sinks:**

```yaml
audit:
  enabled: true
  output: "stdout"
  file_path: "/var/log/mcpproxy/audit.jsonl"
  rotation:
    max_size_mb: 100
    max_age_hours: 168

  # External sinks: fan out to multiple destinations
  sinks:
    # Azure Sentinel (OCSF) — for security monitoring
    - type: ocsf
      enabled: true
      ocsf:
        workspace_id: "{WORKSPACE_ID}"
        api_key: "${AZURE_API_KEY}"
        batch_size: 100
      filter:
        results: ["denied"]  # Only denied events

    # SIEM (CEF over TCP-TLS) — for compliance
    - type: cef
      enabled: true
      cef:
        transport: "tcp_tls"
        host: "siem.company.com"
        port: 6514
      filter:
        methods: ["tools/call"]
```

## Performance

Single static binary, ~12MB. Sub-millisecond proxy overhead:

- p50: ~400us
- p95: ~900us
- Docker image: ~17MB (includes alpine + ca-certificates + tzdata)

Run benchmarks: `./scripts/benchmark.sh`

## Documentation

- [Quick-Start Guide](docs/QUICKSTART.md) — Full setup walkthrough
- [Configuration Reference](docs/CONFIG-REFERENCE.md) — Every YAML field documented
- [Multi-Tenant Setup](docs/MULTI-TENANT.md) — Docker Compose for agencies

## Tests

**276+ tests** across 13 packages with **64–95% coverage** on core packages:

```bash
# Full suite (unit + integration + e2e)
go test ./... -race -count=1

# Coverage report
go test ./... -cover

# Single package
go test ./internal/proxy/ -v -count=1
```

**Test coverage by package:**

| Package | Coverage | Description |
|---|---|---|
| `internal/ratelimit` | 95.5% | Rate limiting (token bucket algorithm) |
| `tests/integration` | 83.3% | Integration tests (5 mock server types) |
| `internal/rbac` | 82.3% | Role-based access control |
| `internal/auth` | 79.8% | OAuth 2.1 PKCE authentication |
| `internal/proxy` | 65.7% | Core proxy pipeline |
| `internal/pii` | 77.3% | PII masking/redaction |

**Test types:**
- **Unit tests**: Individual component tests (auth, rbac, ratelimit, proxy)
- **Integration tests**: 5 mock server types (tools-only, resources, prompts, mixed, streaming)
- **E2E tests**: 24 persona tests covering the full pipeline with buyer personas (Marcus/Priya/James/Sofia/Kai)

**Test coverage highlights:**
- Rate limiter: default config handling, burst behavior, token refresh, cleanup, edge cases (very high/low rates, many unique clients)
- Proxy: multiple upstreams, unknown server fallback, SSE detection with quality values, custom transport injection
- Auth: OAuth flows, PKCE verification, claim-based role mapping, token caching
- RBAC: admin/readonly/restricted roles, per-tool allow/deny lists, batch request handling

## Contributing

PRs welcome. The codebase is standard Go — no frameworks, minimal dependencies.

1. Fork and clone
2. `go test ./...` to verify
3. Make your changes
4. `go test ./...` again
5. Open a PR

## Security

### Security Architecture

The proxy implements defense-in-depth with multiple security layers:

**1. Transport Security**
- TLS 1.2+ required for production (TLS 1.0/1.1 disabled)
- Configurable `tls.required: true` to refuse starting without valid certificates
- Client IP extracted from `X-Forwarded-For` / `X-Real-IP` for accurate logging

**2. Authentication**  
- OAuth 2.1 with PKCE (Proof Key for Code Exchange)
- Supports GitHub, Google, Microsoft Entra ID, and any OIDC provider
- Immutable user identifier (`immutable_id`) from OAuth `sub` claim prevents rate limit bypass
- Token cache with LRU eviction (default 1000 entries, 5-minute TTL)
- Session persistence to disk (optional) survives proxy restarts

**3. Authorization**
- Role-based access control (RBAC) with tool-level granularity
- Three built-in roles: `admin`, `readonly`, `restricted`
- Custom roles supported for fine-grained access control
- Claim-based role mapping from OAuth tokens (enterprise-ready)

**4. Rate Limiting**
- Per-client token bucket (default: 60 RPM, burst 10)
- Uses `immutable_id` from OAuth `sub` claim (not mutable `client_id`)
- HTTP headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

**5. DoS Protection**
- JSON depth limiter (max 32 levels) prevents stack overflow attacks
- Body size limits before JSON parsing
- Graceful degradation under load

**6. Audit & Accountability**
- Immutable JSONL audit trail (tamper-evident)
- Client IP tracked in all audit entries
- Cascading file rotation with configurable `max_backups`
- Multiple external sinks: OCSF (Azure), CEF (SIEM), custom webhooks

**7. Security Hardening**
- CEF sanitization per CEF v0 spec (escapes `\`, `|`, `=`, `:`)
- No secrets in config files (environment variables only)
- Session file permissions: 0600

### Found a vulnerability?

See [SECURITY.md](SECURITY.md) for responsible disclosure.

## License

MIT — completely free and open-source. No tiers, no limits, no hidden fees.

[MIT](LICENSE)
