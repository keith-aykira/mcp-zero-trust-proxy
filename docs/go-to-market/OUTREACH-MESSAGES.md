# MCP Zero-Trust Proxy — Outreach Messages

**Date:** March 19, 2026

Use these templates for Day 2 outreach. Adapt tone to the channel and the person. Always lead with curiosity, not product pitch.

---

## UTM Link Reference

Use the correct tracked link for each channel. Copy the exact URL — do not use the bare domain.

| Channel | UTM Link |
|---------|----------|
| Hacker News (Show HN) | `https://mcpzerotrust.dev/?utm_source=hackernews&utm_medium=show-hn&utm_campaign=launch-2026` |
| Reddit r/aiagents | `https://mcpzerotrust.dev/?utm_source=reddit&utm_medium=community&utm_campaign=launch-2026&utm_content=aiagents` |
| Reddit r/SaaS | `https://mcpzerotrust.dev/?utm_source=reddit&utm_medium=community&utm_campaign=launch-2026&utm_content=saas` |
| MCP Discord | `https://mcpzerotrust.dev/?utm_source=discord&utm_medium=community&utm_campaign=launch-2026&utm_content=mcp-official` |
| Claude Code Discord | `https://mcpzerotrust.dev/?utm_source=discord&utm_medium=community&utm_campaign=launch-2026&utm_content=claude-code` |
| Twitter/X | `https://mcpzerotrust.dev/?utm_source=twitter&utm_medium=social&utm_campaign=launch-2026` |
| DM outreach | `https://mcpzerotrust.dev/?utm_source=dm&utm_medium=outreach&utm_campaign=launch-2026` |
| Email follow-up | `https://mcpzerotrust.dev/?utm_source=email&utm_medium=followup&utm_campaign=launch-2026` |

---

## 1. Discord / Community Post (MCP Discord, Claude Code Community)

### Version A — Question-Led (Recommended First Post)

> **How are you securing your MCP servers?**
>
> I've been setting up MCP servers for a few months now and realized my endpoints have been running with zero auth — just wide open on my network. After reading about the Clawdbot breach and seeing the OWASP MCP Top 10, I started digging into it.
>
> Turns out there are 8,000+ exposed servers out there. The popular OAuth workaround (mcp-remote) had a CVSS 9.6 RCE. Even Anthropic's own Git MCP server had prompt injection flaws that chained into RCE.
>
> I've been building a drop-in auth proxy that sits in front of any MCP server — OAuth 2.1 PKCE, RBAC, session isolation, audit logging. No code changes to your server needed. Docker pull, point it at your MCP endpoint, done.
>
> Curious what everyone else is doing. Are you worried about this? Already have a solution? Would love to hear what's working (or not).
>
> [USE UTM LINK FOR THIS CHANNEL]

### Version B — Show HN Style (For When You Have the MVP)

> **Show: MCP Zero-Trust Proxy — drop-in auth for your MCP servers in 5 minutes**
>
> After the Clawdbot breach exposed 1,800+ servers and researchers found 8,000+ unauthenticated MCP endpoints, I built a reverse proxy that adds OAuth 2.1 PKCE + RBAC + audit logging to any MCP server without code changes.
>
> `docker run -e MCP_TARGET=localhost:3000 mcpproxy/zero-trust`
>
> What it does:
> - OAuth 2.1 PKCE auth (multi-provider: GitHub, Google, Okta, custom)
> - Tool-level RBAC (restrict which tools each client can call)
> - Session isolation (per-client boundaries)
> - Immutable audit log (who called what, when, allowed/denied)
> - Rate limiting per client
>
> Free for 1 server, $49/mo Pro (5 servers), $199/mo Enterprise (unlimited + SSO).
>
> Looking for beta testers — especially teams running 3+ MCP servers. Happy to give free Pro access to early adopters.
>
> [USE UTM LINK FOR THIS CHANNEL]

---

## 2. Direct Messages to Developers

### Template A — To Someone Who Posted About MCP Security

> Hey [Name] — saw your [post/comment] about MCP security in [channel/thread]. Totally resonated — I've been dealing with the same thing.
>
> I'm building a drop-in auth proxy for MCP servers (OAuth 2.1, RBAC, audit logging) that doesn't require changing your server code. Would love to get your take on it.
>
> Quick question: how many MCP servers are you running, and how are you handling auth right now? Even if it's "not at all" — that's useful context.
>
> No pitch, just trying to understand the problem better.
>
> [USE UTM LINK FOR THIS CHANNEL]

### Template B — To Someone Running Exposed MCP Servers

> Hey [Name] — I noticed you're working with MCP servers (saw your [repo/post/config]). Quick question: are you doing anything for auth on those endpoints?
>
> I ask because I've been researching MCP security pretty deeply — there are 8,000+ servers with zero auth out there, and even the popular mcp-remote OAuth wrapper had a CVSS 9.6 RCE bug. Figured I'd check in.
>
> I'm building a proxy that adds OAuth + RBAC to any MCP server with zero code changes. Would you be open to a 10-minute chat about what you'd need from something like that?
>
> [USE UTM LINK FOR THIS CHANNEL]

### Template C — To AI Agency / Team Leads

> Hey [Name] — I see your team is building with [Claude Code / MCP / AI agents]. Curious how you're handling MCP server security as you scale?
>
> The recent CVE wave (30+ in 60 days) and the Clawdbot breach have a lot of teams rethinking their approach. I'm building a zero-trust proxy specifically for MCP — drop-in auth, RBAC, session isolation, full audit trail. No server code changes needed.
>
> Would be great to get 10 minutes of your time to understand your setup and see if this could help. Happy to share what I've learned about the threat landscape either way.
>
> [USE UTM LINK FOR THIS CHANNEL]

---

## 3. Reddit Posts

### r/aiagents — Discussion Starter

> **Title: MCP Security in 2026: What's Your Setup?**
>
> Been doing deep research on MCP server security and the numbers are concerning:
>
> - 8,000+ exposed servers found in Feb 2026
> - Clawdbot breach exposed credentials on 1,800+ servers in 48 hours
> - 30+ CVEs in 60 days (Jan-Feb 2026)
> - The most popular OAuth workaround (mcp-remote) had a CVSS 9.6 RCE
> - OWASP published an MCP Top 10 vulnerability list
> - 75% of MCP servers built by individuals with no centralized security review
>
> For those of you running MCP servers in production: what are you doing for auth? Nginx reverse proxy? Kong? Rolling your own OAuth? Nothing?
>
> I've been building a drop-in proxy for this (zero code changes to your server — just sits in front and handles auth/RBAC/logging). Curious if others see the same pain or if this is overblown.
>
> [USE UTM LINK FOR THIS CHANNEL]

### r/SaaS — Validation Post

> **Title: Building a security product for MCP servers — looking for early feedback**
>
> 10 research reports and 49 scored business ideas later, I'm going all-in on MCP server security.
>
> The thesis: MCP adoption is exploding (97M+ monthly SDK downloads) but authentication is optional in the spec. 41% of production servers have zero auth. The Clawdbot breach, 30+ CVEs in 60 days, and OWASP publishing an MCP Top 10 all validate the timing.
>
> Product: A drop-in reverse proxy that adds OAuth 2.1 PKCE, tool-level RBAC, session isolation, and audit logging to any MCP server. Docker pull, point at your endpoint, done. No code changes.
>
> Pricing: $49/mo starter, $99/mo pro, $199/mo enterprise.
>
> Looking for: Teams running 3+ MCP servers who'd be willing to do a 10-minute discovery call. Free Pro access for beta testers.
>
> What am I missing?
>
> [USE UTM LINK FOR THIS CHANNEL]

---

## 4. Hacker News

### Comment Template (For Relevant Threads)

> The fundamental problem is that MCP's auth is optional. The spec technically supports OAuth 2.1, but implementation is left to individual server authors — and 75% of MCP servers are built by solo developers with no security review process.
>
> After the Clawdbot breach and the CVE wave, I started building a drop-in proxy approach: sits in front of any MCP server and enforces OAuth 2.1 PKCE, RBAC, session isolation, and audit logging without changing server code. Similar to what Microsoft recommended in their February governance blog (API gateway as single enforcement point), but lightweight enough for a solo dev to deploy in 5 minutes.
>
> Would be curious what approaches others have found effective. The "just use nginx" camp seems to underestimate the MCP-specific attack surface (rug pulls, tool squatting, prompt injection chains).
>
> [USE UTM LINK FOR THIS CHANNEL]

---

## 5. Email to Discovery Call Prospects (Post-Call Follow-Up)

> Subject: Following up — MCP auth proxy for [their company/use case]
>
> Hey [Name],
>
> Thanks for taking the time to chat today. Really helpful to hear about your setup — [reference specific detail from call, e.g., "running 5 MCP servers with no auth beyond network segmentation"].
>
> As promised, here's the quick summary:
>
> **MCP Zero-Trust Proxy** adds OAuth 2.1 PKCE, tool-level RBAC, session isolation, and audit logging to any MCP server — zero code changes, single Docker container.
>
> Based on what you described, the [specific pain point] seems like the most pressing issue. I'll have the beta ready by [date]. I'd love to have you as one of our first testers — free Pro access for the first 3 months.
>
> [USE UTM LINK FOR THIS CHANNEL]
>
> Happy to jump on another call when the beta's ready if you want to walk through it together.
>
> Best,
> Andrew

---

## Tips for Outreach

1. **Lead with curiosity, not pitch.** Ask how they handle security. Listen first.
2. **Reference specific data.** "8,000 exposed servers" and "CVSS 9.6" are concrete and credible.
3. **Mention the Clawdbot breach by name.** It's the most recognizable incident in this space.
4. **Offer value regardless of conversion.** Share the threat landscape data. Be helpful.
5. **Track everything.** Note who responds, their setup, their objections. This feeds Week 1 synthesis.
6. **Don't over-message.** One DM per person. One community post per channel. Follow up only if they engage.
7. **Always use the correct UTM link for each channel.** See the UTM Link Reference table at the top of this file.
