# MCP Zero-Trust Proxy

A drop-in reverse proxy that adds zero-trust security to any [MCP](https://modelcontextprotocol.io) server — no code changes required.

```bash
docker run -e MCP_TARGET=localhost:3000 -e AUTH_PROVIDER=github -p 8080:8080 \
  ghcr.io/anoblescm/mcp-zero-trust-proxy
```

## What it does

MCP (Model Context Protocol) is how AI agents connect to tools — Claude, Cursor, Copilot all use it. But authentication is optional in the spec. This proxy sits between your MCP clients and servers to enforce security:

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

### Docker (recommended)

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
  client_secret: "\${OAUTH_CLIENT_SECRET}"
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

## How it works

```
AI Client (Claude, Cursor, Copilot)
        │
        ▼
┌─────────────────────────┐
│  MCP Zero-Trust Proxy   │
│                         │
│  1. Body size check     │
│  2. Auth (OAuth 2.1)    │
│  3. Rate limit          │
│  4. JSON-RPC parse      │
│  5. RBAC check          │
│  6. Forward to upstream │
│  7. Filter tools/list   │
│  8. Audit log           │
└─────────────────────────┘
        │
        ▼
   Your MCP Server
   (unchanged)
```

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

236+ tests across 10 packages (including new cmd/mcpproxy tests):

```bash
go test ./...
go test -race ./...  # zero data races
```

## Contributing

PRs welcome. The codebase is standard Go — no frameworks, minimal dependencies.

1. Fork and clone
2. `go test ./...` to verify
3. Make your changes
4. `go test ./...` again
5. Open a PR

## Security

Found a vulnerability? See [SECURITY.md](SECURITY.md) for responsible disclosure.

## License

MIT — completely free and open-source. No tiers, no limits, no hidden fees.

[MIT](LICENSE)
