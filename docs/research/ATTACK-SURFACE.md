# MCP Zero-Trust Proxy — Attack Surface & Threat Landscape Research

**Date:** March 19, 2026 | **Classification:** Confidential

---

## 1. The Clawdbot Breach (January 2026)

**What happened:** Clawdbot (rebranded as Moltbot/OpenClaw), a popular open-source AI assistant, experienced a catastrophic security incident between January 23–26, 2026. MCP endpoints had no mandatory authentication and default configurations bound admin panels to `0.0.0.0:8080` (publicly accessible).

**Scale:**
- 1,862 exposed servers identified by Knostic
- 17,903 OpenClaw gateways exposed on Shodan
- 900+ Clawdbot instances exposed (separate report)

**Data at risk:** Full agent conversation histories, environment variables with API keys, database credentials, internal service tokens, system prompts, user data, and agent control functions.

**Active exploitation:** RedLine, Lumma, and Vidar infostealers added Clawdbot to their target lists within 48 hours — before most security teams even knew it was running in their environments.

**Sources:**
- VentureBeat: venturebeat.com/security/mcp-shipped-without-authentication-clawdbot-shows-why-thats-a-problem
- VentureBeat: venturebeat.com/security/clawdbot-exploits-48-hours-what-broke
- Guardz: guardz.com/blog/when-ai-agents-go-wrong-clawdbots-security-failures
- Acuvity: acuvity.ai/the-clawdbot-dumpster-fire-72-hours-that-exposed-everything-wrong-with-ai-security

---

## 2. Exposed MCP Servers (8,000+ in February 2026)

**The claim:** 8,000+ MCP servers with public internet exposure discovered by security researchers (February 2026).

**Source:** Nyami's Medium article "8,000+ MCP Servers Exposed: The Agentic AI Security Crisis of 2026" (cikce.medium.com)

**Broader context:**
- Zuplo's February 2026 survey found 17,000+ MCP server listings across directories
- AgentSeal scanned 1,808 MCP servers — 66% had security findings (agentseal.org)
- Enkrypt AI scanned 1,000 MCP servers — 33% had critical vulnerabilities (enkryptai.com)
- 42,665 exposed instances identified in one comprehensive scan
- 41% of 518 surveyed production servers had NO authentication

---

## 3. Named CVEs and Incidents (January–March 2026)

### CVE-2025-6514 (mcp-remote) — CVSS 9.6
- **First documented full RCE against an MCP client in production**
- 437,000+ downloads affected
- Affects Claude Desktop, VS Code, Cursor
- Server-provided OAuth endpoints trusted without validation; embedded commands executed during auth
- Source: jfrog.com/blog/2025-6514-critical-mcp-remote-rce-vulnerability

### CVE-2026-27825 & 27826 (mcp-atlassian) — CVSS 9.1 / 8.2
- 4.4K GitHub stars, 4M+ downloads
- Path traversal → arbitrary file write → RCE and privilege escalation
- SSRF via unvalidated header parsing
- Source: blog.pluto.security/p/mcpwnfluence-cve-2026-27825-critical

### CVE-2025-68143/68144/68145 (Anthropic Git MCP Server)
- Anthropic quietly patched three prompt injection flaws in their official Git MCP server
- `git_init` accepted arbitrary paths, initializing repos in sensitive directories
- Combined with Filesystem MCP → write malicious git config → RCE via shell hook
- git_init tool removed entirely in version 2025.12.18
- Source: theregister.com/2026/01/20/anthropic_prompt_injection_flaws

### SmartLoader Campaign (February 2026)
- Attackers cloned legitimate Oura MCP server, built fake GitHub accounts/forks
- Injected StealC infostealer targeting developers (API keys, cloud creds, crypto wallets)
- Months of credibility building before payload deployment
- Source: thehackernews.com/2026/02/smartloader-attack-uses-trojanized-oura

### GitHub MCP Private Repo Access
- Agent hijacking allows exfiltration of private repository data
- Architectural issue — not a code flaw
- Source: invariantlabs.ai/blog/mcp-github-vulnerability

### Asana MCP Data Exposure (June 2025)
- Data exposed across account boundaries
- Non-malicious but demonstrated architectural weaknesses
- Source: upguard.com/blog/asana-discloses-data-exposure-bug-in-mcp-server

**Total CVE count:** 30+ CVEs filed in 60 days (January–February 2026)

---

## 4. OWASP MCP Guidance (February 16, 2026)

OWASP Gen AI Security Project published "A Practical Guide for Secure MCP Server Development" and "OWASP MCP Top 10" vulnerability list.

**Key recommendations:** Secure architecture patterns, strong authentication/authorization enforcement, strict input validation, session isolation, hardened deployment.

**Sources:**
- genai.owasp.org/resource/a-practical-guide-for-secure-mcp-server-development/
- owasp.org/www-project-mcp-top-10/

---

## 5. Palo Alto Unit 42: MCP Rug Pull Attacks

Unit 42 documented attacks where threat actors compromise open-source MCP server repositories and modify server functionality post-deployment. Users who auto-update receive backdoored versions unknowingly.

**Source:** unit42.paloaltonetworks.com/navigating-security-tradeoffs-ai-agents/

---

## 6. Microsoft MCP Governance (February 12, 2026)

Microsoft published secure-by-default MCP architecture:
- Every remote MCP server must sit behind API gateway (single point of auth, rate-limiting, logging)
- Vetted server lists — unapproved connections receive friendly nudge with registration path
- Short-lived, least-privilege tokens with proof-of-possession
- Living inventory of connected services

**Source:** microsoft.com/insidetrack/blog/protecting-ai-conversations-at-microsoft-with-model-context-protocol-security-and-governance/

---

## 7. Developer Sentiment

**Adoption pressure:** 97M+ monthly MCP SDK downloads. 96% of IT leaders say agents improve employee experience. 85% of enterprises implementing AI agents by end of 2025. $1.8B market.

**Security fear:** 43% of MCP servers have OAuth flaws. 75% of servers built by individuals with no vendor accountability. Blocking MCP creates shadow AI deployments.

**Common workarounds:** OAuth proxies (vulnerable — CVE 9.6 in mcp-remote), reverse proxies (Nginx with TLS), vetted registries (Microsoft approach), least-privilege tokens, API gateways.

**Sources:**
- Zuplo MCP Report: zuplo.com/mcp-report
- Trace3: blog.trace3.com/the-mcp-security-maturity-gap

---

## Key Narratives for Marketing

1. **"8,000+ exposed MCP servers"** — concrete proof of problem
2. **"Clawdbot exposed credentials in 48 hours"** — real-time threat urgency
3. **"30 CVEs in 60 days"** — vulnerability wave
4. **"OWASP, Microsoft, Unit 42 all released guidance in February 2026"** — establishment validation
5. **"CVSS 9.6 RCE in the most popular OAuth workaround"** — workarounds don't work
6. **"75% of MCP servers built by individuals with no accountability"** — structural risk
7. **"97M monthly downloads but 41% have zero auth"** — adoption outpacing security
