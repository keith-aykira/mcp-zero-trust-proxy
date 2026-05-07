# AGENTS.md — Agent Instructions

## Commands

```bash
# Build
make build
go build -o bin/mcpproxy ./cmd/mcpproxy

# Full pipeline (lint → test → build)
make all
# Or manually:
go vet ./...
go test ./... -race -count=1
go build -o bin/mcpproxy ./cmd/mcpproxy

# Test (all 9 packages, 236+ tests)
go test ./... -v -race -count=1

# Test single package
go test ./internal/proxy/ -v
go test ./tests/integration/ -v
go test ./tests/e2e/ -v

# Docker
docker build -t mcpzerotrust/proxy .
docker run -p 8080:8080 -v ./config.yaml:/etc/mcpproxy/config.yaml mcpzerotrust/proxy
```

## Architecture

**Pipeline order** (critical for debugging):
1. Body size check → 2. Auth (OAuth 2.1 PKCE) → 3. Rate limit → 4. JSON-RPC parse → 5. RBAC check → 6. Forward → 7. Filter tools/list → 8. Audit log

**MCP methods**: `initialize`, `tools/list`, `tools/call`, `resources/read`, `prompts/get`

**Go modules**: Only 4 direct deps — `zerolog`, `golang.org/x/time/rate`, `gopkg.in/yaml.v3`, plus stdlib. No external JWT library.

## Package Map

```
cmd/mcpproxy/main.go  — Entry point, wires auth/rbac/proxy/ratelimit/audit
internal/
  auth/    — OAuth 2.1 PKCE (providers: github, google, oidc), session management, claim-based role mapping
  proxy/   — HTTP handler, SSE, JSON-RPC pipeline, CORS
  rbac/    — Role engine (admin/readonly/restricted), tool filtering middleware
  ratelimit/ — Token bucket limiter (per-client, default: 60 RPM, burst 10)
  audit/   — JSONL logger with rotation
  config/  — YAML loader with ${ENV_VAR} substitution
tests/
  integration/ — 20+ tests with 5 mock server types
  e2e/       — 24 persona tests (validates full pipeline scenarios)
```

## Gotchas

- **User-role mapping uses email keys**: Config must use `"alice@co.com": "admin"` not username
- **`go test ./...` from repo root**: Running from subdirectory fails due to module path
- **Rate limit state is per-client session**: Not global — different clients can each hit limits
- **RBAC filters tools/list response**: Content-Length is recalculated after filtering (`proxy/pipeline.go`)
- **Docker image uses multi-stage build**: Final stage has no shell, just binary
- **Auth middleware sets `ctx value=user`**: Downstream code retrieves via `r.Context().Value(authCtxKey)`

## Release

Tag `vx.y.z` → GitHub Actions builds Docker image to GHCR + builds release binaries for 4 platforms.

```bash
git tag v1.0.0 && git push origin v1.0.0
```

Binary size limit enforced in CI: must be <20MB.

## Config

- Env var substitution: `${VARIABLE_NAME}` in YAML
- Secrets always in env, never in YAML
- Default config path: `configs/example.yaml` for reference
- `--config` flag overrides default

## Testing Notes

- Race detector runs on every test: `-race`
- `-count=1` prevents test caching (integration tests use real HTTP servers)
- E2E tests validate 5 buyer personas (Marcus/Priya/James/Sofia/Kai)
- Integration tests spawn mock MCP servers on ephemeral ports

## Conventions

- Conventional commits: `feat:`, `fix:`, `docs:`, `chore:`, `test:`
- Config via YAML, secrets via env vars only
- No code changes to protected MCP server (proxying only)
