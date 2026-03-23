# Open-Source vs Closed-Source Decision

**Date:** 2026-03-23
**Status:** Closed-source for v1.0. Review at 90-day milestone.
**Requirement:** PLH-09

## The Question

Should the MCP Zero-Trust Proxy core be open-sourced before or after the HN launch?

## Arguments for Open-Source

1. **Trust for a security product** — developers are deeply skeptical of closed-source security tools. Open code is auditable code. HN audience will ask "how do I know it does what you say?"
2. **Community contributions** — MCP ecosystem is moving fast. Open-source attracts contributors who add new providers, fix bugs, and improve docs.
3. **Discoverability** — GitHub stars, forks, and issues drive organic discovery. Closed repos don't appear in GitHub search.
4. **Competitive moat is not the code** — the proxy's value is in the product, docs, and managed licensing. Anyone could write a similar Go proxy; the hard part is the distribution and trust.
5. **Freemium flywheel** — open-source free tier with paid Pro/Enterprise is a proven SaaS model (HashiCorp, PostHog, Grafana).

## Arguments Against Open-Source

1. **Competitive copying** — a funded competitor could fork, rebrand, and undercut on price before we establish brand recognition.
2. **License complexity** — Business Source License (BSL) or AGPL required to prevent commercial forks; adds friction for enterprise buyers.
3. **Support burden** — open issues, PRs, and community questions require time. Solo founder bandwidth is limited.
4. **Premature** — the product hasn't been validated with paying customers yet. Open-sourcing before product-market fit adds noise.

## Decision

**Closed-source for v1.0 launch.** Review after 90-day beta validation.

**Rationale:** The trust concern is real but addressable without full open-source — through reproducible builds, the private security review offer, and honest FAQ copy. The competitive copying risk is meaningful before we have brand recognition.

**Trigger for revisiting:** If HN comments show significant distrust due to closed source, or if 3+ enterprise prospects cite it as a blocker, open-source the core proxy under AGPL.

**If we open-source:** Open core (proxy, RBAC, audit) under AGPL. Keep Stripe billing integration, license key system, and landing page closed. This is the PostHog model.
