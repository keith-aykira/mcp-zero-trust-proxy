# Information Classification

This guide covers the optional information classification system — a layer of access control that sits on top of RBAC, enforcing classification-based tool access with declassification prevention.

## Overview

The classification system adds government-grade information security controls to your MCP proxy. It lets you:

1. **Define a classification hierarchy** — e.g., `Public` → `Sensitive` → `Confidential`
2. **Assign tools to classification levels** — which tools are public, which are classified
3. **Cap role clearance levels** — which roles may access which classification tiers
4. **Prevent declassification** — once a session reaches a classification level, it cannot drop below it, preventing information leakage from high to low contexts

Classification is **opt-in** and **backward compatible**. When no `classification` block is configured, the proxy behaves exactly as before.

| Feature | Description |
|---|---|
| **Classification levels** | Ordered hierarchy defining clearance tiers |
| **Tool assignments** | Map each tool to a classification level |
| **Role clearance** | Cap maximum level per role |
| **Session floor** | Floor rises with access, never drops — prevents declassification |
| **tools/list filtering** | Classified tools hidden from users below clearance |

---

## How It Works

### Evaluation Order

When a `tools/call` request arrives, the proxy evaluates access in this order:

```
1. deny_tools check          → explicit deny always wins
2. allow_tools check         → restricted roles checked against whitelist
3. classification level      → role's max level vs tool's level
4. session floor check       → current session floor vs tool's level
```

If any step fails, the request is denied with HTTP 403 / JSON-RPC error `-32002`.

### Classification Levels

Levels are defined as an ordered list from lowest to highest. The first element is the lowest classification; the last element is the highest.

```yaml
classification:
  levels:
    - "Public"
    - "Sensitive"
    - "Confidential"
```

A role with clearance at `Sensitive` may access tools classified `Public` or `Sensitive`, but not `Confidential`.

### Tool Assignments

Each tool is mapped to a classification level. Unassigned tools default to the **lowest level** (e.g., `Public`).

```yaml
classification:
  tool_assignments:
    "read_public_report":    "Public"
    "query_internal_metrics": "Sensitive"
    "view_billing_data":     "Confidential"
```

### Role Clearance

Set `classification_level` on a role to cap its maximum accessible classification. Omitting this field leaves the role **unconstrained by classification** — it bypasses level checks.

```yaml
roles:
  - name: "admin"
    classification_level: "Confidential"
  - name: "analyst"
    classification_level: "Sensitive"
  - name: "intern"
    # no classification_level → unconstrained (bypasses classification)
```

### Session Floor & Declassification Prevention

The session floor is the core security feature. Once a user calls a tool at a given classification level, their session floor rises to that level. They can **never call a tool below that floor** again in the same session.

This prevents a dangerous pattern: a user reads confidential data, then sends it to a lower-classification endpoint that might log, forward, or expose the data differently.

```
Session starts:              floor = Public (level 0)
User calls Sensitive tool:   floor → Sensitive (level 1)
User calls Public tool:      DENIED (Public < Sensitive floor)
User calls Confidential:     floor → Confidential (level 2)  [if role allows]
User calls Sensitive tool:   DENIED (Sensitive < Confidential floor)
```

The floor only rises. It never drops during the session.

---

## Configuration

### Minimal Configuration

Classification is inactive until **both** `levels` and `tool_assignments` are configured.

```yaml
# Classification is INACTIVE — no levels + no assignments
classification:
  levels:
    - "Public"
    - "Sensitive"
```

### Active Configuration

```yaml
classification:
  levels:
    - "Public"
    - "Sensitive"
    - "Confidential"

  tool_assignments:
    "read_file":      "Public"
    "search_files":   "Public"
    "query_db":       "Sensitive"
    "view_billing":   "Confidential"
```

With this config, a role at `Sensitive` clearance can call `read_file`, `search_files`, and `query_db`, but not `view_billing`.

### Full Example

```yaml
# Classification hierarchy (lowest → highest)
classification:
  levels:
    - "Public"
    - "Internal"
    - "Confidential"
    - "Secret"

  # Tool-to-level assignments
  tool_assignments:
    "list_projects":       "Public"
    "read_documentation":  "Public"
    "query_internal_metrics": "Internal"
    "view_employee_records":  "Confidential"
    "access_source_code":     "Secret"

# Role with clearance cap
roles:
  - name: "admin"
    classification_level: "Secret"
    allowed_tools: []

  - name: "analyst"
    classification_level: "Internal"
    allowed_tools:
      - "list_projects"
      - "read_documentation"
      - "query_internal_metrics"

  - name: "intern"
    # No classification_level → bypasses classification checks entirely
    allowed_tools:
      - "list_projects"
      - "read_documentation"
```

---

## Behavior Details

### When Classification Is Active

Classification is active when **both** of the following are true:

1. `levels` is non-empty
2. `tool_assignments` is non-empty

If either is absent, classification is a no-op and the proxy behaves as if no classification is configured.

### Default Levels

When `classification` is present but `levels` is not explicitly set, the defaults are:

```
["Public", "Sensitive", "Confidential"]
```

### Unassigned Tools

Tools not listed in `tool_assignments` are treated as the lowest classification level (the first entry in `levels`).

### Roles Without `classification_level`

A role that omits `classification_level` is **unconstrained** by classification. It passes the classification check regardless of the tool's assigned level. This enables gradual rollout: add classifications to some roles first, then expand.

### tools/list Filtering

The `tools/list` response is filtered at two layers:

1. **RBAC layer** — restricted roles only see tools in their `allowed_tools` list
2. **Classification layer** — tools above the role's clearance are removed

Additionally, once a session floor has risen, tools **below the floor** are also removed from the list — keeping the tools visible aligned with what the user can actually call.

### Custom Hierarchy Names

You may use any names for classification levels. They are not restricted to standard names. Examples:

- Military: `Unclassified`, `Confidential`, `Secret`, `Top Secret`
- Corporate: `Public`, `Internal`, `Confidential`, `Restricted`
- Simplified: `Low`, `Medium`, `High`

---

## Example Scenarios

### Scenario 1: Analyst Accesses Metrics, Then Tries Public Endpoint

```
Role:        analyst (clearance: Internal)
Session ID:  abc-123

1. tools/call → query_internal_metrics  [Internal]  → ALLOWED, floor: Internal
2. tools/call → list_projects           [Public]    → DENIED  (Public < Internal floor)
```

The analyst called an Internal tool first. Their floor rose. They can no longer call Public tools — preventing data from Internal context leaking through a Public endpoint.

### Scenario 2: Escalation Path

```
Role:        admin (clearance: Secret)
Session ID:  def-456

1. tools/call → list_projects          [Public]     → ALLOWED, floor: Public
2. tools/call → query_internal_metrics [Internal]   → ALLOWED, floor: Internal
3. tools/call → view_employee_records  [Confidential] → ALLOWED, floor: Confidential
4. tools/call → access_source_code     [Secret]     → ALLOWED, floor: Secret
5. tools/call → list_projects          [Public]     → DENIED  (Public < Secret floor)
```

Escalation is allowed. De-escalation is prevented.

### Scenario 3: Separate Sessions Are Independent

```
Session A: calls Confidential tool → floor: Confidential
Session B: calls Public tool       → ALLOWED (floor was Public)
```

Session floors are tracked per `session_id`. One session's floor does not affect another.

### Scenario 4: Unclassified Role Bypasses

```
Role:        intern (no classification_level set)
Session ID:  ghi-789

1. tools/call → access_source_code     [Secret]     → ALLOWED (role unconstrained)
2. tools/call → list_projects          [Public]     → ALLOWED
```

The `intern` role has no classification cap and is unconstrained. Classification checks are skipped. (RBAC `allow_tools` / `deny_tools` still apply.)

---

## Security Considerations

### Why Declassification Prevention Matters

Without session floor tracking, an attacker (or compromised account) could:

1. Read confidential data via a high-classification tool
2. Send that same data to a low-classification tool that forwards to an external service
3. The external service logs, caches, or shares the data

The session floor prevents step 2 by ensuring the session stays at the highest classification level reached.

### Session Management

The session floor is tied to the proxy's session store (default TTL: 24 hours). When a session expires, the floor resets for the next session. If you need tighter control, configure a shorter session TTL in your auth settings.

### Interaction with RBAC

Classification is **additional** to, not a replacement for, RBAC. The `deny_tools` and `allow_tools` lists still apply. A tool must pass **all** checks:

```
denied? → blocked
not in allow list? → blocked
above role clearance? → blocked
below session floor? → blocked
→ forwarded to upstream
```

### Audit Logging

Classification-related denials are logged in the audit trail with `result: "denied"` and include the tool name, role, and session ID. Review audit logs to monitor classification enforcement.

---

## Validation

The config validator checks:

- No duplicate level names in `levels`
- All `tool_assignments` reference valid levels
- All role `classification_level` values reference valid levels
- Level names are unique across the hierarchy

Invalid configuration errors at startup with descriptive messages.

---

## FAQ

**Q: Can I change classification levels after deployment?**
A: Yes, but rolling a new config will reset session floors. Active sessions will have stale floor state until they expire.

**Q: Does classification affect `resources/read` or `prompts/get`?**
A: No. Classification currently applies only to `tools/call`. It gates tool invocation and `tools/list` filtering.

**Q: How do I debug classification denials?**
A: Check audit logs for `result: "denied"` entries. The entry will show the role, tool, and session ID. Verify the role's `classification_level` against the tool's assigned level and the session's current floor.

**Q: Can I have overlapping classification and RBAC restrictions?**
A: Yes, and it's recommended. Use `allow_tools`/`deny_tools` for coarse-grained access, and classification for fine-grained, level-based control.
