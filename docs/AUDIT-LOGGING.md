# Audit Logging

## Overview

The MCP Zero-Trust Proxy provides comprehensive audit logging of all client interactions including `initialize`, `tools/list`, `resources/read`, and `tools/call` events. Audit logs capture detailed information about who accessed what, when, with what parameters, and what the outcome was.

Built on an event-driven architecture with JSONL formatting, the audit system supports multiple simultaneous output destinations through a pluggable sink framework.

---

## Audit Event Structure

Every audit event is a single JSON object written as one line:

```json
{
  "timestamp": "2025-01-01T00:00:00.000000000Z",
  "event": {
    "session_id": "uuid",
    "client_id": "client:service:account",
    "user_id": "user-uuid",
    "method": "tools/call",
    "tool": "read_file",
    "params": "... or null",
    "latency_ms": 42,
    "result": "success",
    "error": "..."
  }
}
```

### Fields Reference

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string (RFC3339) | Server time of event |
| `session_id` | string (UUID) | Unique session identifier |
| `client_id` | string | OAuth `client_id` (e.g., `client:service:account`) |
| `user_id` | string | OAuth `sub` claim |
| `method` | string | MCP method: `initialize`, `tools/list`, `resources/read`, `tools/call` |
| `tool` | string | Tool name (for `tools/call` events) |
| `params` | string\|null | Truncated/escaped request parameters (first 1KB) |
| `latency_ms` | integer | Request latency in milliseconds |
| `result` | string | One of: `success`, `denied`, `error` |
| `error` | string | Error message (when `result` is `error`) |
| `denied_reason` | string | RBAC denial reason (when `result` is `denied`) |

---

## Output Configuration

### Basic Configuration

```yaml
audit:
  enabled: true
  output: "stdout"  # stdout, stderr, file, both, off
  file_path: "/var/log/mcpproxy/audit.jsonl"
```

### File Output with Rotation

```yaml
audit:
  enabled: true
  output: "file"
  file_path: "/var/log/mcpproxy/audit.jsonl"
  rotation:
    max_size_mb: 100     # Max file size before rotation
    max_age_hours: 168   # Max age of old files (7 days)
```

**Output Modes:**
- `stdout` — Write to standard output (default, useful for containers)
- `stderr` — Write to standard error
- `file` — Write to file with optional rotation
- `both` — Write to both stdout and file
- `off` — Disable stdout/file output (use external sinks only)

---

## Audit Sinks Framework

The proxy supports multiple external audit sinks that can run in parallel with the built-in stdout/file output. Events are fanned out to all enabled sinks.

### Available Sinks

| Sink | Use Case | Protocol |
|------|----------|----------|
| **OCSF** | Azure Sentinel, Microsoft security tools | OCSF 12.0.0 over HTTP |
| **CEF** | SIEM systems (Splunk, QRadar, etc.) | Syslog protocols |
| **JSON-HTTP** | Custom webhooks, internal services | HTTPS JSON |
| **File** | Local file output with rotation | File I/O |

### Sink Filtering

Each sink can filter events independently to reduce noise and costs:

```yaml
filter:
  methods: ["tools/call"]              # Only log tool calls
  tools: ["read_file", "write_file"]   # Specific tools
  results: ["denied"]                  # Only denials
  client_ids: ["client:service:x"]     # Specific clients
```

---

### OCSF Sink (Azure Sentinel)

**Configuration:**

```yaml
audit:
  sinks:
    - type: "ocsf"
      enabled: true
      name: "azure-sentinel"
      ocsf:
        workspace_id: "{WORKSPACE_ID}"
        api_key: "${INVOKE_TOKEN}"     # Or log_analytics_key
        batch_size: 100                # Batch events (default: 100)
        batch_timeout_ms: 1000         # Max wait time (default: 1s)
      filter:
        results: ["denied"]            # Only log denials
```

**Features:**
- Native integration with Azure Monitor/Sentinel via OCSF 12.0.0
- Batching for efficiency (100 events or 1 second)
- Async delivery with buffering (up to 1000 in-flight events)
- Automatic field mapping to OCSF security event schema

**Environment Variables:**
- `AZURE_LOG_ANALYTICS_WORKSPACE_ID` — Workspace ID
- `INVOKE_TOKEN` — HTTP API key (preferred)
- `LOG_ANALYTICS_KEY` — Primary/secondary key

**Note:** Uses HTTP POST API by default. Falls back to `LogAnalytics-TokenType: bearer` header if `INVOKE_TOKEN` is not set.

---

### CEF Sink (Syslog/SIEM)

**Configuration:**

```yaml
audit:
  sinks:
    - type: "cef"
      enabled: true
      name: "siem-forwarder"
      cef:
        transport: "tcp_tls"           # udp, tcp, tcp_tls, https
        host: "siem.company.com"
        port: 6514                    # Default: 514 (UDP/TCP), 443 (TLS/HTTPS)
        device_id: "mcpproxy"         # CEF DeviceID
        severity: 4                   # 1-10 (default: 4=warning)
        source_address: "10.0.0.1"    # Optional source IP
        facility: 16                  # Syslog facility (default: local0)
      filter:
        methods: ["tools/call", "resources/read"]
```

**Transports Supported:**
- `udp` — UDP syslog (port 514)
- `tcp` — TCP syslog (port 514)
- `tcp_tls` — TCP with TLS (port 6514, rsyslog standard)
- `https` — HTTPS with syslog-ng HTTP endpoint

**CEF Format Example:**

```
CEF:0|mcpproxy|MCP Proxy|1.0.0|0|Tool Access|4|Sub=1 src=10.0.0.1 act=tools/call tool=read_file result=success latency=42 user=john@company.com client=client:service:account
```

**Fields Mapped:**
- `Sub` — User ID
- `src` — Source address (if configured)
- `act` — Method name
- `tool` — Tool name (for `tools/call`)
- `result` — success/denied/error
- `latency` — Request latency in ms
- `user` — User ID
- `client` — Client ID

---

### JSON-HTTP Sink (Custom Webhooks)

**Configuration:**

```yaml
audit:
  sinks:
    - type: "json_http"
      enabled: true
      name: "internal-processor"
      json_http:
        url: "https://audit-processor.internal/api/events"
        headers:
          Authorization: "Bearer ${AUDIT_API_KEY}"
          X-Team: "platform"
        timeout_ms: 5000
        max_retries: 3
        retry_backoff_ms: 100
      filter:
        methods: ["tools/call"]
```

**Use Cases:**
- Forward to internal audit processing services
- Send to message queues (Kafka, RabbitMQ via HTTP bridge)
- Integrate with custom compliance workflows
- Feed real-time dashboards

**Features:**
- Custom HTTP headers with environment variable substitution
- Configurable timeout and retry logic
- JSON payload matches the standard audit event format
- Async delivery with backpressure

---

### File Sink (External)

**Configuration:**

```yaml
audit:
  output: "off"  # Disable built-in stdout/file
  sinks:
    - type: "file"
      enabled: true
      name: "audit-log"
      file:
        path: "/var/log/mcpproxy/audit.jsonl"
        max_size_mb: 100
        max_age_hours: 168
      filter:
        results: ["denied", "error"]
```

**Use Case:** When you need filtered output separate from the main log stream.

---

## Multiple Sinks Example

All output streams can run simultaneously:

```yaml
audit:
  enabled: true
  output: "both"                          # stdout + file
  file_path: "/var/log/mcpproxy/audit.jsonl"
  rotation:
    max_size_mb: 100
    max_age_hours: 168
  
  sinks:
    # Azure Sentinel — security monitoring
    - type: "ocsf"
      enabled: true
      name: "azure-sentinel"
      ocsf:
        workspace_id: "{WORKSPACE_ID}"
        api_key: "${INVOKE_TOKEN}"
      filter:
        results: ["denied"]               # Only denials
  
    # SIEM — compliance logging
    - type: "cef"
      enabled: true
      name: "siem-compliance"
      cef:
        transport: "tcp_tls"
        host: "siem.company.com"
        port: 6514
      filter:
        methods: ["tools/call"]           # Only tool calls
  
    # Internal processor — analytics
    - type: "json_http"
      enabled: true
      name: "analytics"
      json_http:
        url: "https://analytics.internal/audit"
        headers:
          Authorization: "Bearer ${ANALYTICS_KEY}"
      filter:
        methods: ["tools/call"]           # Tool call analytics
```

---

## Performance Considerations

### Throughput

- **Local file:** 10,000+ events/second
- **OCSF (batched):** ~1,000 events/second (network-dependent)
- **CEF (TCP-TLS):** ~2,000 events/second
- **JSON-HTTP:** ~500-1,000 events/second

### Buffering

- OCSF: Batches up to 100 events or 1 second (configurable)
- All sinks: Buffer up to 1000 in-flight events
- Drops events only if backpressure exceeds buffer (logs warning)

### Recommendations

1. **Use filtering** to reduce external sink load:
   ```yaml
   filter:
     results: ["denied"]  # Only log what matters
   ```

2. **Batch when possible** — OCSF sink batches automatically

3. **Separate concerns** — Different sinks for different purposes:
   - File: Full debug logging
   - OCSF: Security monitoring (denials only)
   - CEF: Compliance reporting

4. **Monitor drop rates** — Check logs for "dropping audit events" warnings

---

## Troubleshooting

### Verify Output

**Check stdout:**
```bash
./mcp-zero-trust-proxy --config config.yaml 2>&1 | grep "session_id"
```

**Check file:**
```bash
tail -f /var/log/mcpproxy/audit.jsonl | jq .
```

### Common Issues

**No events appearing:**
- Check `audit.enabled: true`
- Verify `output` is not `off` (or you have sinks configured)
- Check filters are not too restrictive

**OCSF sink not working:**
- Verify `workspace_id` is in format `{WORKSPACE_ID}`
- Check `INVOKE_TOKEN` or `LOG_ANALYTICS_KEY` is set
- Test with: `curl -X POST https://oauth.loganalytics.io/v1/ingest/{workspace_id}/events HTTP/1.1`

**CEF sink connection refused:**
- Verify SIEM is listening on the correct port
- Check firewall rules for UDP/TCP traffic
- Confirm TLS certificate is valid (for tcp_tls/https)

**High latency:**
- Reduce `batch_size` for faster delivery
- Increase `batch_timeout_ms` for better throughput
- Check network connectivity to external sinks

---

## Security Considerations

- **Credentials:** Use environment variables for sensitive values
- **TLS:** Always use TLS for external sinks in production
- **Filtering:** Filter at source to minimize data exposure
- **Access Control:** Restrict network access to audit endpoints
- **Retention:** Configure appropriate retention in sink systems

---

## Integration Examples

### Azure Sentinel Query

```kusto
SecurityEvent
| where Name == "Tools Call Event"
| where Result_s == "denied"
| summarize count() by UserFullName_s, Tool_s
| order by count_ desc
```

### Splunk Query

```splunk
index=siem product="mcpproxy" action="tools/call"
| stats count by user, tool, result
| where result="denied"
```

### Real-time Dashboard (Grafana)

```json
{
  "datasource": "azuremonitor",
  "query": "SecurityEvent | where Name == \"Tools Call Event\" | summarize count() by bin(TimeGenerated, 5m)"
}
```

---

## Changelog

### v1.1.0

- Added pluggable sink framework with OCSF, CEF, JSON-HTTP, and file sinks
- Added event filtering per sink
- Added batching for OCSF sink
- Changed `latency` field to `latency_ms` (integer, milliseconds)
- Added `denied_reason` field for RBAC denials
- Support for multiple simultaneous output destinations
