# Quick-Start Guide

MCP Zero-Trust Proxy is a free, open-source reverse proxy that sits in front of any MCP server and adds zero-trust security — OAuth 2.1 PKCE authentication, role-based access control (RBAC), per-client rate limiting, and a tamper-evident audit trail — with zero code changes to the MCP server.

You configure one YAML file, run one Docker command, and every request to your MCP server is authenticated, authorized, and logged.

---

## Prerequisites

- **Docker** (recommended), or **Go 1.22+** for binary installation
- An MCP server running locally or accessible over the network
- An OAuth provider account: GitHub or Google (or any OIDC-compliant provider)

---

## Option A: Docker (recommended)

### 1. Pull the image

```bash
docker pull ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
```

Or build from source:

```bash
git clone https://github.com/AnobleSCM/mcp-zero-trust-proxy.git
cd mcp-zero-trust-proxy
docker build -t mcp-zero-trust-proxy .
```

### 2. Create config.yaml

Create a `config.yaml` in your current directory. At minimum you need `upstream_url`:

```yaml
server:
  upstream_url: "http://host.docker.internal:3000"  # your MCP server
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
  - name: "restricted"
    allowed_tools: []

audit:
  enabled: true
  output: "stdout"
```

See [configs/example.yaml](../configs/example.yaml) for the full set of options.

### 3. Run the proxy

```bash
docker run \
  -p 8080:8080 \
  -v ./config.yaml:/etc/mcpproxy/config.yaml \
  -e OAUTH_CLIENT_SECRET=your_actual_secret \
  ghcr.io/anoblescm/mcp-zero-trust-proxy:latest \
  --config /etc/mcpproxy/config.yaml
```

Or with Docker Compose using the provided `docker-compose.yaml`:

```bash
OAUTH_CLIENT_SECRET=your_actual_secret docker compose up
```

### 4. Verify the proxy is running

```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

---

## Option B: Binary

### 1. Install

**Using `go install`:**

```bash
go install github.com/AnobleSCM/mcp-zero-trust-proxy/cmd/mcpproxy@latest
```

**Or download a release binary** from the [GitHub releases page](https://github.com/AnobleSCM/mcp-zero-trust-proxy/releases) and place it on your PATH.

### 2. Create config.yaml

Same as the Docker option above. The binary looks for `./config.yaml` by default, or pass `--config` to specify a path:

```bash
mcpproxy --config /path/to/config.yaml
```

### 3. Start the proxy

```bash
export OAUTH_CLIENT_SECRET=your_actual_secret
mcpproxy --config ./config.yaml
```

### 4. Verify

```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

---

## Configure your MCP client

Once the proxy is running, point your MCP client at the proxy instead of directly at the MCP server.

**Claude Desktop** — edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "my-server": {
      "url": "http://localhost:8080"
    }
  }
}
```

Change `"url"` from the MCP server's address to `http://localhost:8080` (the proxy).

**Cursor** — update the server URL in Cursor's MCP settings to `http://localhost:8080`.

**Any MCP client** — replace the direct server URL with the proxy URL. The proxy is transparent to the MCP protocol.

---

## Set up an OAuth provider

### GitHub (recommended for quick start)

1. Go to [GitHub Developer Settings](https://github.com/settings/developers) → OAuth Apps → New OAuth App
2. Fill in:
   - **Application name:** any name (e.g., "MCP Proxy")
   - **Homepage URL:** `http://localhost:8080`
   - **Authorization callback URL:** `http://localhost:8080/auth/callback`
3. Click "Register application"
4. Copy the **Client ID**
5. Click "Generate a new client secret" and copy it
6. Set in your config:
   ```yaml
   auth:
     provider: "github"
     client_id: "your-copied-client-id"
     client_secret: "${GITHUB_CLIENT_SECRET}"
     redirect_url: "http://localhost:8080/auth/callback"
   ```
7. Run with `GITHUB_CLIENT_SECRET=your-copied-secret mcpproxy --config ./config.yaml`

### Google

1. Go to [Google Cloud Console](https://console.cloud.google.com/) → APIs & Services → Credentials
2. Click "Create Credentials" → OAuth client ID
3. Application type: **Web application**
4. Add to "Authorized redirect URIs": `http://localhost:8080/auth/callback`
5. Copy the Client ID and Client Secret
6. Set in your config:
   ```yaml
   auth:
     provider: "google"
     client_id: "your-google-client-id"
     client_secret: "${GOOGLE_CLIENT_SECRET}"
     redirect_url: "http://localhost:8080/auth/callback"
   ```

### Custom OIDC provider

```yaml
auth:
  provider: "oidc"
  client_id: "your-client-id"
  client_secret: "${OIDC_CLIENT_SECRET}"
  issuer_url: "https://your-auth-server.example.com"
  redirect_url: "http://localhost:8080/auth/callback"
```

---

## Configure RBAC

The proxy has three built-in roles: `admin`, `readonly`, and `restricted`.

| Role | What they can do |
|------|------------------|
| `admin` | Call any tool, read any resource, use any prompt |
| `readonly` | List tools/resources/prompts, read resources — but cannot call tools that modify state |
| `restricted` | Call only the tools explicitly listed in `allowed_tools` |

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

**Configuring the restricted role:**

```yaml
roles:
  - name: "restricted"
    allowed_tools:
      - "read_file"
      - "list_directory"
      - "search_files"
    deny_tools: []
```

A restricted client making a `tools/list` call will only see the tools in their `allowed_tools` list — the proxy filters the response. A `tools/call` for a tool not in the list returns HTTP 403 with a JSON-RPC error.

**Explicitly denying tools:**

Any role can deny specific tools using `deny_tools`. Deny rules take precedence over allow rules:

```yaml
roles:
  - name: "admin"
    allowed_tools: []
    deny_tools:
      - "delete_database"
      - "execute_arbitrary_code"
```

---

## Verify the full flow

**1. Attempt an unauthenticated call (should get 401):**

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
# Expected: HTTP 401 with JSON-RPC error code -32001
```

**2. Authenticate via OAuth:**

Open `http://localhost:8080/auth/start` in a browser. Complete the OAuth flow with your GitHub/Google account. You'll receive a bearer token.

**3. Make an authenticated call:**

```bash
curl -X POST http://localhost:8080 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
# Expected: HTTP 200 with your MCP server's tool list
```

**4. Check the audit log:**

The proxy writes one JSONL line per request to stdout (by default):

```json
{"timestamp":"2025-01-15T10:30:00Z","client_id":"user@example.com","method":"tools/list","allowed":true,"latency_ms":12,"request_id":"abc123"}
```

---

## Troubleshooting

### 502 Bad Gateway / 504 Gateway Timeout

If the proxy returns `502` or `504`, the upstream MCP server is unreachable or too slow:

- **502:** The upstream server refused the connection or returned an invalid response. Check that `upstream_url` in your config points to a running MCP server.
- **504:** The upstream server didn't respond within the timeout window. The proxy uses a 120-second write timeout and 30-second read timeout. If your MCP server needs longer for expensive tool calls, place the proxy behind a load balancer with extended timeouts.

```bash
# Quick check: is the upstream reachable?
curl -s http://localhost:3000/health  # replace with your upstream URL
```

### SSE connection drops

MCP uses Server-Sent Events (SSE) for streaming. If a client disconnects (browser tab closed, network interruption), the SSE connection is terminated. The proxy does not automatically reconnect — the client must re-establish the connection. This is standard SSE behavior and matches the MCP specification.

If you see frequent SSE drops, check for reverse proxies or load balancers between the client and the proxy that may be timing out idle connections. Set their idle timeout higher than your longest expected tool call duration.

### Rate limiting behavior

Rate limits are enforced **per proxy instance**. If you run multiple replicas behind a load balancer, each instance maintains its own rate limit counters independently. This means a client's effective rate limit is multiplied by the number of instances.

For most deployments (single instance or small clusters), this is the correct behavior. If you need globally coordinated rate limiting across many replicas, use an external rate limiter (e.g., Redis-backed) in front of the proxy.

---

## Next steps

- **Full configuration reference:** [CONFIG-REFERENCE.md](CONFIG-REFERENCE.md) — documents every YAML field with types, defaults, and examples
- **Rate limiting:** Set `rate_limit.requests_per_minute` to control per-client throughput (default: 300 req/min)
- **Audit log to file:** Set `audit.output: "file"` and `audit.file_path: "/var/log/mcpproxy/audit.jsonl"` for persistent logs
- **Production TLS:** Place a TLS-terminating reverse proxy (nginx, Caddy, Cloudflare Tunnel) in front of the proxy for HTTPS
- **Multi-client / agency setup:** [MULTI-TENANT.md](MULTI-TENANT.md) — how to run separate proxy instances per client with Docker Compose
- **Monitoring:** The proxy exposes `/health` for uptime checks (returns `{"status":"ok"}`). For request-level metrics, parse the JSONL audit log with Filebeat, Fluentd, or Vector to feed Elasticsearch, Datadog, or Splunk. A Prometheus `/metrics` endpoint is on the roadmap.
