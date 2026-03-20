# MCP Ecosystem Pulse (Mar 2026)

**Research date:** 2026-03-19 | **Source:** Exa deep_researcher_pro

---

## 1. SDK Downloads

### PyPI (Python)
| Month | `mcp` downloads | `fastmcp` downloads |
|-------|----------------|-------------------|
| Oct 2025 | ~54.5M | ~23.5M |
| Nov 2025 | ~59.8M (peak) | ~25.0M (peak) |
| Dec 2025 | ~57.2M | ~24.2M |
| Jan 2026 | ~56.0M | ~23.8M |
| Feb 2026 | ~52.0M | ~22.0M |
| Mar 2026 (partial) | ~12.0M | ~5.0M |

**Trend:** Late 2025 peak → gradual decline into 2026. Not collapsing but normalizing from hype spike.

### npm (TypeScript)
- @modelcontextprotocol/sdk: Daily trend chart shows similar pattern — late 2025 peak, softer into Mar 2026
- Monthly totals must be derived from daily data (npm-stat.com)

### Interpretation
Adoption is in a **hype-to-normalization cycle**, not declining. Feb 2026 still at ~52M monthly PyPI downloads — that's massive absolute volume. The ecosystem is settling into stable usage patterns.

---

## 2. Server Counts

| Registry | Count | Notes |
|----------|-------|-------|
| Smithery | ~3,993 | As of Mar 19, 2026 |
| Official MCP Registry | Paginated API, no total published | Active registrations visible |
| Ecosystem-wide estimate | ~10,000+ | Pento blog (late 2025 review) |

**Trend:** Thousands of servers across multiple registries. Growth continuing.

---

## 3. Protocol Spec Changes

Key updates in the MCP specification:

- **OAuth 2.1 authorization framework added to core spec** — formalizes auth expectations for servers/clients
- **Streamable HTTP transport** — supersedes HTTP+SSE pattern
- **JSON-RPC batching** — improved request efficiency
- **Enhanced tool annotations** — read-only vs destructive indicators
- **Audio data support** added
- **Completions capability** for argument autocompletion

**Critical nuance on OAuth 2.1:** The spec now includes auth as a standard, but it's NOT mandatory — implementations can still skip it. This standardizes the interface but doesn't enforce security. Our proxy remains valuable as enforcement + RBAC + audit on top of the spec's optional auth.

---

## 4. Enterprise Adoption Signals

| Company | Signal | Date |
|---------|--------|------|
| Atlassian | Rovo MCP Server GA | Feb 12, 2026 |
| PayPal | MCP for agent efficiency + context | Q1 2026 |
| Entro Security | Enterprise Agentic Governance & Administration | Mar 18, 2026 |
| SurePath AI | Real-time MCP policy controls | Mar 12, 2026 |
| Microsoft | Azure Foundry MCP auth docs + SQL MCP Server | Q1 2026 |
| Google Cloud | MCP Toolbox Java SDK + Cloud SQL integration | Mar 3, 2026 |

**Assessment:** Enterprise adoption is ACCELERATING. Fortune-scale companies are productizing MCP access controls, which validates the market and creates demand for governance/security tooling.

---

## 5. Platform Support

Platforms with native MCP support as of Mar 2026:
- **Anthropic Claude** (Claude Code, Claude Desktop)
- **OpenAI ChatGPT** (Agent Builder / Agents)
- **Microsoft Copilot** (Copilot Studio)
- **Cursor**
- **Windsurf**
- **n8n** integrations
- **LangChain** agent runtimes

**Assessment:** MCP is now the de facto standard for AI tool integration. No competing protocol has emerged.

---

## 6. Developer Activity (GitHub)

| Repo | Stars | Notes |
|------|-------|-------|
| modelcontextprotocol/servers | ~77.3K | Massive community |
| modelcontextprotocol/modelcontextprotocol (spec) | ~7.1K | ~334 contributors |
| modelcontextprotocol/go-sdk | ~3.8K | Active development |

**Assessment:** Very healthy community engagement. Stars and contributor counts indicate sustained interest.

---

## 7. Anthropic/OpenAI Auth Announcements

- **Anthropic:** Published agent eval content, shipped Claude Code with MCP integration. NO announcement of a platform-wide built-in auth gateway.
- **OpenAI:** No MCP auth announcements found in Jan-Mar 2026.
- **Neither** has announced plans to ship a complete auth/gateway that would eliminate third-party security products.

**Key insight:** The spec added OAuth 2.1 as a STANDARD, but platforms are not shipping ENFORCEMENT. Auth is standardized but optional — the gap between "auth is available" and "auth is enforced" is exactly our product.

---

## Kill Criteria Assessments

### "MCP spec or Anthropic/OpenAI is adding built-in auth that eliminates the need"
**VERDICT: NO.** OAuth 2.1 was added to the spec but remains optional. Neither Anthropic nor OpenAI has announced a built-in auth gateway. The spec standardizes auth flows but doesn't enforce them — enforcement is our product.

### "MCP adoption is stalling (flat or declining SDK downloads)"
**VERDICT: NO — BUT WATCH CLOSELY.** Downloads are normalizing from a late-2025 hype peak (~60M/mo → ~52M/mo on PyPI). This is expected stabilization, not a crash. 52M monthly downloads is enormous absolute volume. Enterprise adoption signals (Atlassian, PayPal, Microsoft, Google) are accelerating even as raw downloads moderate.

---

*Sources: pypistats.org, npm-stat.com, Smithery.ai, MCP specification changelog, Anthropic/OpenAI public materials, GitHub repos*
