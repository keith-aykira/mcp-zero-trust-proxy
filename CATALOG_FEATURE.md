# Tool Catalog Feature

## Overview

The tool catalog feature provides persistent caching of MCP server tool metadata in a SQLite database, with periodic background updates. This enables:

1. **Faster tools/list responses**: Serve cached tool lists without forwarding to upstream servers
2. **User-specific tool filtering**: Return only tools the user has permission to access
3. **Improved reliability**: Cached data available even when upstream servers are temporarily unavailable
4. **Reduced upstream load**: Fewer tools/list requests forwarded to upstream servers

## Architecture

### Components

1. **Catalog Package** (`internal/catalog/catalog.go`)
   - SQLite database for persistent storage
   - Background goroutine for periodic tool list refresh
   - Methods for querying tools by server and filtering by permissions

2. **Pipeline Integration** (`internal/proxy/pipeline.go`)
   - Intercepts `tools/list` requests
   - Serves cached responses when catalog is enabled
   - Applies RBAC filtering to cached tool lists

3. **Configuration** (`internal/config/types.go`)
   - Catalog settings in YAML configuration
   - Database path, refresh interval, cache enable/disable flags

### Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Background Sync Loop                      │
│                                                              │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐               │
│  │ Server A │    │ Server B │    │ Server C │               │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘               │
│       │ tools/list   │ tools/list   │ tools/list            │
│       ▼              ▼              ▼                       │
│  ┌─────────────────────────────────────────────┐           │
│  │           SQLite Database                    │           │
│  │  ┌──────────────────────────────────────┐   │           │
│  │  │  tools table                         │   │           │
│  │  │  - server_name (index)               │   │           │
│  │  │  - name (index)                      │   │           │
│  │  │  - description                       │   │           │
│  │  │  - input_schema                      │   │           │
│  │  │  - updated_at                        │   │           │
│  │  └──────────────────────────────────────┘   │           │
│  └─────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
                        ▲
                        │ query (filtered by server + RBAC)
                        │
┌─────────────────────────────────────────────────────────────┐
│                    Request Pipeline                          │
│                                                              │
│  tools/list request ──► Check catalog cache ──► Return      │
│                                    │                        │
│                                    └─► Miss/error ──► Proxy │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

Add to your YAML configuration:

```yaml
catalog:
  enabled: true
  database_path: "./tools_catalog.db"  # Default: ./tools_catalog.db
  cache_tools_list: true               # Serve tools/list from cache
```

### Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable catalog feature |
| `database_path` | string | `"./tools_catalog.db"` | SQLite database file path |
| `cache_tools_list` | bool | `false` | Serve tools/list from cache instead of forwarding |

## Database Schema

```sql
CREATE TABLE tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_name, name)
);

CREATE INDEX idx_tools_server ON tools(server_name);
CREATE INDEX idx_tools_name ON tools(name);
```

## Usage Examples

### Basic Setup

```yaml
server:
  registry:
    servers:
      - name: "files"
        url: "http://localhost:3001"
      - name: "database"
        url: "http://localhost:3002"

catalog:
  enabled: true
  database_path: "./tools_catalog.db"
  cache_tools_list: false  # Still forward to upstream, but cache for future
```

### Full Caching Mode

```yaml
catalog:
  enabled: true
  database_path: "./tools_catalog.db"
  cache_tools_list: true  # Serve directly from cache
```

### With RBAC Filtering

```yaml
roles:
  - name: "restricted"
    allowed_tools:
      - "files.read_file"
      - "files.list_directory"

catalog:
  enabled: true
  cache_tools_list: true

# Users with "restricted" role will only see allowed tools in tools/list
```

## Behavior

### Cache Hit (cache_tools_list: true)

1. User sends `tools/list` request
2. Pipeline checks catalog cache
3. Catalog returns filtered tool list:
   - Filtered by server name
   - Filtered by user's RBAC permissions
4. Response served directly from cache (no upstream call)

### Cache Miss or Disabled

1. User sends `tools/list` request
2. Request forwarded to upstream server
3. Response captured and filtered by RBAC (if enabled)
4. Response returned to user
5. Background sync updates catalog periodically

### Background Sync

```
Every <refresh_interval>:
  For each configured server:
    Fetch tools/list from upstream
    Upsert into database (delete old, insert new)
```

## Performance Considerations

### Benefits

- **Reduced latency**: Cached responses avoid upstream round-trip
- **Lower upstream load**: Fewer tools/list requests
- **Better availability**: Cached data available during upstream outages

### Trade-offs

- **Cache freshness**: Tools may be stale between sync intervals
- **Memory usage**: SQLite database grows with number of tools
- **Storage**: Database file persists on disk

## Verification

Check catalog is working:

```bash
# Check database exists
ls -la tools_catalog.db

# Check tool count
sqlite3 tools_catalog.db "SELECT COUNT(*) FROM tools;"

# Check servers
sqlite3 tools_catalog.db "SELECT DISTINCT server_name FROM tools;"

# Check recent updates
sqlite3 tools_catalog.db "SELECT server_name, name, updated_at FROM tools ORDER BY updated_at DESC LIMIT 10;"
```

## Future Enhancements

- [ ] Configurable refresh interval per server
- [ ] HTTP API for manual cache invalidation
- [ ] Metrics for cache hit/miss ratio
- [ ] Support for resources and prompts cataloging
- [ ] Tool metadata aggregation across servers
- [ ] Search/query interface for cataloged tools
