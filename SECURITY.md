# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in MCP Zero-Trust Proxy, please report it responsibly.

**Email:** support@mcpzerotrust.dev

Include as much of the following as possible:

- Description of the vulnerability
- Steps to reproduce
- Affected version(s)
- Potential impact

## Response Timeline

- **Acknowledgment:** Within 48 hours of your report
- **Fix target:** Within 14 days of confirmed vulnerability
- **Disclosure:** Coordinated with reporter after fix is released

## Scope

The following are in scope for security reports:

- Authentication bypass (OAuth 2.1 PKCE flow)
- Authorization bypass (RBAC policy enforcement)
- Audit log bypass or tampering
- Rate limit bypass
- Data leakage across client sessions
- Injection via JSON-RPC parsing

## Out of Scope

- Denial of service attacks
- Social engineering
- Vulnerabilities in upstream MCP servers (not controlled by this proxy)
- Issues in dependencies that are already publicly disclosed (report upstream)

## Credit

Reporters will be credited in release notes unless they prefer to remain anonymous. Let us know your preference when reporting.

## Supported Versions

Security fixes are applied to the latest release only. We recommend always running the most recent version.
