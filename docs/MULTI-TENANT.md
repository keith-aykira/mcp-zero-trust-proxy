# Multi-Client / Agency Setup

MCP Zero-Trust Proxy uses one upstream MCP server per proxy instance. If you manage MCP servers for multiple clients, run a separate proxy per client.

This is the recommended architecture for data isolation — each client gets its own proxy process, its own config, its own audit trail, and its own rate limits.

---

## Docker Compose Example

```yaml
services:
  proxy-client-a:
    image: ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
    ports: ["8081:8080"]
    volumes: ["./configs/client-a.yaml:/etc/mcpproxy/config.yaml"]
    environment:
      OAUTH_CLIENT_SECRET: ${CLIENT_A_OAUTH_SECRET}

  proxy-client-b:
    image: ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
    ports: ["8082:8080"]
    volumes: ["./configs/client-b.yaml:/etc/mcpproxy/config.yaml"]
    environment:
      OAUTH_CLIENT_SECRET: ${CLIENT_B_OAUTH_SECRET}

  proxy-client-c:
    image: ghcr.io/anoblescm/mcp-zero-trust-proxy:latest
    ports: ["8083:8080"]
    volumes: ["./configs/client-c.yaml:/etc/mcpproxy/config.yaml"]
    environment:
      OAUTH_CLIENT_SECRET: ${CLIENT_C_OAUTH_SECRET}
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

This project is MIT licensed and free to use. No license keys required.

---

## FAQ

**Can one proxy serve multiple upstreams?**
Not currently. The `upstream_url` config field accepts a single URL. One proxy = one upstream. This keeps the security model simple — each client's data is isolated at the process level.



**How do I know which client a request came from?**
The audit log includes `client_id` (the authenticated user's email). Each proxy instance only serves one client's upstream, so the source container also identifies the client.
