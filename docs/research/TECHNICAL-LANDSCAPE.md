# Technical Security Landscape Update (Feb–Mar 2026)

**Research date:** 2026-03-19 | **Source:** Exa deep_researcher_pro

---

## 1. New CVEs (Feb–Mar 2026)

| CVE | CVSS | Component | Impact | Status |
|-----|------|-----------|--------|--------|
| CVE-2026-4270 | Important (AWS) | AWS API MCP Server | File access restriction bypass — exposes local files to MCP client | Fixed >= 1.3.9 |
| CVE-2026-27826 | 8.2 (High) | mcp-atlassian | SSRF via header — credential theft, network scanning, prompt injection chains | Patch available |
| CVE-2026-25536 | 7.1 (High) | MCP TypeScript SDK | Cross-client data leak via race condition in transport reuse | Fixed >= 1.26.0 |
| CVE-2026-27735 | 6.4 (Medium) | mcp-server-git | Path traversal in git_add — staging files outside repo boundaries | Fixed >= 2026.1.14 |
| CVE-2026-25650 | 7.5 (High) | MCP Salesforce Connector | Arbitrary attribute access — Salesforce auth token disclosure | Fixed in 0.1.10 |
| CVE-2026-27825 | 9.1 (Critical) | mcp-atlassian | Arbitrary file write → RCE, chainable with SSRF (CVE-2026-27826) | Patch available |

**Key insight:** The MCP TypeScript SDK itself (CVE-2026-25536) had a data leak vulnerability — this isn't just bad server implementations, it's the core SDK.

---

## 2. MCP Spec Auth Updates

- **No mandatory authentication merged into the official MCP spec** as of March 19, 2026
- Community/vendor proposals for "SMCP" (Secure MCP) describe signed tool descriptors, encrypted transport defaults, mandatory tool permission declarations — but these remain proposals, not spec changes
- The official MCP repo (github.com/model-context-protocol/mcp) had no auth-specific PRs or issues opened/merged/closed in Feb–Mar 2026

**Key insight:** The spec gap remains wide open. No mandatory auth = our product thesis is intact.

---

## 3. Platform Security Announcements

### Microsoft (ACTIVE)
- Azure Foundry Agent Service: full MCP authentication docs (key-based, Entra/Azure AD, OAuth passthrough)
- Dev blog: production MCP server with OAuth 2.1 + Azure AD (token validation, OBO flows)
- SQL MCP Server connector with IAM and RBAC controls

### Google Cloud (ACTIVE)
- MCP Toolbox Java SDK (Mar 3, 2026) — handles auth integration for Cloud SQL, AlloyDB
- Cloud SQL auto-enabling remote MCP servers with IAM deny policies
- Database-specific MCP access controls

### Anthropic
- SMCP proposal (community-level) — not an official product
- Security guidance published but no built-in auth product shipped

### OpenAI
- No MCP auth announcements found in Feb–Mar 2026

**Key insight:** Microsoft and Google are building auth into their MANAGED MCP offerings. This narrows the enterprise market but leaves the self-hosted/multi-cloud/open-source market wide open.

---

## 4. New Attack Vectors

| Vector | Description | Source |
|--------|-------------|--------|
| Tool poisoning | Tampering with MCP tool metadata to influence agent tool selection | Named attack vector (Feb 2026) |
| Schema injection / Rug Pull | Dynamic modification of tool schemas after handshake to inject adversarial instructions | Deconvolute Labs |
| Indirect prompt injection via MCP | Hidden instructions in tool data executed by agents processing untrusted inputs | StackOne |
| Supply-chain campaigns (SANDWORM_MODE) | Malicious npm packages installing rogue MCP servers/connectors | Dev.to (Feb 2026) |
| Combined chaining | SSRF → arbitrary file write → RCE (demonstrated in mcp-atlassian) | Pluto Security |
| Exposed enterprise surface | Thousands of publicly reachable MCP servers enabling code execution + data exfiltration | eSecurity Planet |

**Key insight:** Attack surface is EXPANDING. Both classic web vulns AND new AI-native vectors. Tool poisoning and schema injection are brand-new attack classes.

---

## 5. Industry Standards

- **OWASP Agentic Security Initiative**: Top-10 style guidance for agent apps (covers Tool Poisoning, Identity & Privilege Abuse, Supply Chain Vulnerabilities)
- **NIST**: No MCP-specific guidance. Preliminary draft of "Cyber AI Profile" published (general AI cybersecurity framework)
- **CIS/ISO/IEC**: No MCP-specific standards published
- **SOC 2 (2026)**: Emphasizes continuous risk assessment and supply chain governance, AI governance as emerging audit area — but doesn't name MCP specifically

**Key insight:** Compliance frameworks are moving toward AI agent security requirements. First movers who provide auditable controls will win.

---

## 6. Regulatory Movement

- **EU AI Act**: Active enforcement but doesn't mention MCP by name. Risk-based controls apply to MCP deployments materially.
- **SOC 2**: 2026 updates emphasize vendor management and supply chain risk for AI systems, but no MCP-specific callouts.
- **NIST AI Agent Standards Initiative**: Washington beginning to focus on autonomous AI governance.

**Key insight:** Regulators are framing requirements around AI agents broadly. MCP-specific mandates will follow. Being audit-ready now = competitive advantage.

---

## 7. New Open-Source Security Tools

| Tool | What it does | When |
|------|-------------|------|
| Obot MCP Gateway | Catalog/discovery + governance for MCP servers | Feb 2026 |
| Sage | Protective security layer between AI agents and OS | Mar 2026 |
| Allama | AI security automation platform for agentic threats | Feb 2026 |
| OpenClaw Scanner | Detects autonomous AI agents in corporate environments | Feb 2026 |
| Pompelmi | Secure file-upload scanning for Node.js (MCP file inputs) | Feb 2026 |

---

## 8. Net Assessment

**Threat landscape: GETTING WORSE**
- Multiple high/critical CVEs in MCP servers, connectors, AND the core SDK
- New AI-native attack classes (tool poisoning, schema injection, supply chain)
- Thousands of exposed servers remain

**Platform mitigation: PARTIAL**
- Microsoft and Google building auth into managed offerings
- But: no spec-level mandatory auth, heterogeneous third-party servers remain unprotected
- Self-hosted, multi-cloud, and open-source MCP deployments still need third-party protection

**Bottom line:** Market opportunity remains strong. Platforms are solving auth for THEIR managed customers, but the long tail of MCP servers (open source, self-hosted, multi-cloud) is growing faster than platform security coverage.

---

*Sources: AWS Security Bulletin, NVD, Pluto Security, SentinelOne, Microsoft Learn, Google Cloud Blog, OWASP, NIST, EU Parliament, and 40+ additional sources via Exa deep_researcher_pro*
