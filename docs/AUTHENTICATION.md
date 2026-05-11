# Authentication Guide

This guide covers all authentication options for securing your MCP Zero-Trust Proxy.

## Overview

The proxy supports multiple authentication providers via OAuth 2.1 with PKCE:

| Provider | Type | Best For |
|----------|------|----------|
| **GitHub** | OAuth 2.0 | Development teams using GitHub |
| **Google** | OIDC | Google Workspace organizations |
| **Microsoft Entra ID** | OIDC | Azure/Entra environments |
| **Generic OIDC** | OIDC | Custom identity providers (Okta, Auth0, Keycloak, etc.) |

---

## GitHub OAuth

**Quick setup for development teams.**

### Configuration

```yaml
auth:
  provider: "github"
  client_id: "Iv1.YOUR_CLIENT_ID"
  client_secret: "${GITHUB_CLIENT_SECRET}"
  redirect_url: "http://localhost:8080/auth/callback"
```

### Setup Steps

1. Go to **GitHub Settings** → **Developer settings** → **OAuth Apps**
2. Click **New OAuth App**
3. Fill in:
   - **Application name**: MCP Zero-Trust Proxy
   - **Homepage URL**: `http://localhost:8080` (or your production domain)
   - **Authorization callback URL**: `http://localhost:8080/auth/callback`
4. Click **Register application**
5. Copy the **Client ID** (starts with `Iv1.`)
6. Generate a new **Client Secret** and copy it
7. Add `user:email` scope to your app settings (required for email access)

### Scopes Requested

- `read:user` - Read user profile
- `user:email` - Access user email address

---

## Google OAuth

**For Google Workspace organizations.**

### Configuration

```yaml
auth:
  provider: "google"
  client_id: "YOUR_PROJECT_ID.apps.googleusercontent.com"
  client_secret: "${GOOGLE_CLIENT_SECRET}"
  redirect_url: "http://localhost:8080/auth/callback"
```

### Setup Steps

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select or create a project
3. Navigate to **APIs & Services** → **Credentials**
4. Click **Create Credentials** → **OAuth 2.0 Client ID**
5. Choose **Web application**
6. Add **Authorized JavaScript origins**: `http://localhost:8080`
7. Add **Authorization redirect URIs**: `http://localhost:8080/auth/callback`
8. Click **Create** and copy the credentials

### Scopes Requested

- `openid` - OpenID Connect authentication
- `email` - Access user email
- `profile` - Access user profile

---

## Microsoft Entra ID (Azure AD)

**For Microsoft Entra (formerly Azure AD) environments - Single or multi-tenant.**

### Single-Tenant Configuration

```yaml
auth:
  provider: "entra"
  client_id: "YOUR_CLIENT_ID"
  client_secret: "${ENTRA_CLIENT_SECRET}"
  issuer_url: "https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0"
  redirect_url: "http://localhost:8080/auth/callback"
```

### Multi-Tenant Configuration

```yaml
auth:
  provider: "entra"
  client_id: "YOUR_CLIENT_ID"
  client_secret: "${ENTRA_CLIENT_SECRET}"
  issuer_url: "https://login.microsoftonline.com/common/v2.0"
  redirect_url: "http://localhost:8080/auth/callback"
```

### Setup Steps

#### 1. Register Application in Entra ID

1. Go to [Microsoft Entra admin center](https://entra.microsoft.com/)
2. Navigate to **Identity** → **Applications** → **App registrations**
3. Click **New registration**
4. Fill in:
   - **Name**: MCP Zero-Trust Proxy
   - **Supported account types**:
     - **Single-tenant**: "Accounts in this organizational directory only"
     - **Multi-tenant**: "Accounts in any organizational directory (any Azure AD tenant - Multitenant)"
   - **Redirect URI**:
     - Type: **Web**
     - URI: `http://localhost:8080/auth/callback`
5. Click **Register**

#### 2. Get Tenant ID and App ID

After registration, note:
- **Application (client) ID** → Use as `client_id` in config
- **Directory (tenant) ID** → Use in `issuer_url`

#### 3. Create Client Secret

1. In your app registration, click **Certificates & secrets**
2. Click **New client secret**
3. Add a description (e.g., "MCP Proxy")
4. Choose expiration period
5. Click **Add**
6. **Copy the secret value immediately** (it won't be shown again)

#### 4. Configure API Permissions

1. Click **API permissions**
2. Click **Add a permission** → **Microsoft Graph**
3. Select **Delegated permissions**
4. Add:
   - `openid`
   - `profile`
   - `email`
5. Click **Add permissions**
6. Click **Grant admin consent** (if available)

### Tenant ID Options

| Value | Description |
|-------|-------------|
| `GUID` | Your specific tenant ID (use for single-tenant) |
| `common` | Any Azure AD tenant (multi-tenant, legacy accounts only) |
| `organizations` | Any Azure AD tenant (multi-tenant, org accounts only) |
| `consumers` | Microsoft personal accounts only |
| `tenant-name.onmicrosoft.com` | Tenant name (works like using GUID) |

### Scopes Requested

- `openid` - OpenID Connect authentication
- `email` - Access user email
- `profile` - Access user profile

### User Claims

Entra ID provides additional claims you can use for role mapping:

| Claim | Description |
|-------|-------------|
| `tid` | Tenant ID |
| `oid` | Object ID |
| `upn` | User principal name (email format) |
| `email` | User email address |
| `name` | Display name |
| `groups` | Group memberships (if configured) |

---

## Generic OIDC Provider

**For Okta, Auth0, Keycloak, or any OIDC-compliant provider.**

### Okta Configuration

```yaml
auth:
  provider: "oidc"
  client_id: "YOUR_CLIENT_ID"
  client_secret: "${OKTA_CLIENT_SECRET}"
  issuer_url: "https://YOUR_ORG.okta.com/oauth2/default"
  redirect_url: "http://localhost:8080/auth/callback"
```

### Auth0 Configuration

```yaml
auth:
  provider: "oidc"
  client_id: "YOUR_CLIENT_ID"
  client_secret: "${AUTH0_CLIENT_SECRET}"
  issuer_url: "https://YOUR_DOMAIN.us.auth0.com/"
  redirect_url: "http://localhost:8080/auth/callback"
```

### Keycloak Configuration

```yaml
auth:
  provider: "oidc"
  client_id: "YOUR_CLIENT_ID"
  client_secret: "${KEYCLOAK_CLIENT_SECRET}"
  issuer_url: "https://keycloak.example.com/realms/YOUR_REALM"
  redirect_url: "http://localhost:8080/auth/callback"
```

### Setup Steps (Generic OIDC)

1. Create/register a new **Client** or **Application** in your OIDC provider
2. Set **Application type** to **Web** or **Server**
3. Configure **Redirect/Callback URL**: `http://localhost:8080/auth/callback`
4. Set **Authorization flow** to **Authorization Code** (with PKCE)
5. Add scopes: `openid`, `email`, `profile`
6. Note the **Client ID** and **Client Secret**
7. Find the **Issuer URL** format (usually documented in provider's OIDC docs)

### Finding Issuer URLs

| Provider | Issuer URL Format |
|----------|------------------|
| Okta | `https://{org}.okta.com/oauth2/{auth_server_id}` |
| Auth0 | `https://{domain}.us.auth0.com/` |
| Keycloak | `https://{host}/realms/{realm}` |
| PingIdentity | `https://{host}/oauth2/rs` |
| ForgeRock | `https://{host}/realms/{realm}/.well-known/openid-configuration` |

---

## Role Mapping with Entra ID

Entra ID supports claim-based role mapping for automatic role assignment.

### Example: Engineering Team Gets Admin Access

```yaml
auth:
  provider: "entra"
  client_id: "${ENTRA_CLIENT_ID}"
  client_secret: "${ENTRA_CLIENT_SECRET}"
  issuer_url: "https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0"
  redirect_url: "http://localhost:8080/auth/callback"

user_roles:
  default: "readonly"
  claim_mapping:
    # Users in "mcp-admins" Entra group get admin role
    - claim: "groups"
      operator: "contains"
      value: "mcp-admins"
      role: "admin"
    
    # Users with engineering email prefix get devops role
    - claim: "email"
      operator: "starts_with"
      value: "eng-"
      role: "devops"
```

### Available Operators

| Operator | Description |
|----------|-------------|
| `equals` | Exact match |
| `contains` | Substring match |
| `starts_with` | Prefix match |
| `ends_with` | Suffix match |
| `regex` | Regular expression match |

---

## Verification

### Test Authentication Flow

```bash
# Start the proxy
docker run -p 8080:8080 \
  -e AUTH_PROVIDER=entra \
  -e AUTH_CLIENT_ID=your_client_id \
  -e AUTH_ISSUER_URL=https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0 \
  -e AUTH_CLIENT_SECRET=your_secret \
  ghcr.io/anoblescm/mcp-zero-trust-proxy:latest

# Start OAuth flow
curl -I http://localhost:8080/auth/start

# You should see: HTTP 302 redirect to provider login page
```

### Debug OIDC Discovery

For OIDC providers, you can manually verify the discovery document:

```bash
# Entra ID discovery
curl -s "https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0/.well-known/openid-configuration" | jq .

# Okta discovery  
curl -s "https://YOUR_ORG.okta.com/oauth2/default/.well-known/openid-configuration" | jq .
```

The response should contain `authorization_endpoint`, `token_endpoint`, and `userinfo_endpoint`.

---

## Security Considerations

### Production Configuration

```yaml
auth:
  provider: "entra"
  client_id: "${ENTRA_CLIENT_ID}"
  client_secret: "${ENTRA_CLIENT_SECRET}"  # Always use env vars for secrets
  issuer_url: "https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0"
  redirect_url: "https://your-production-domain.com/auth/callback"
```

### Key Recommendations

1. **Use HTTPS in production** - All redirect URLs should use HTTPS
2. **Restrict redirect URIs** - Only add necessary callback URLs
3. **Use environment variables** - Never commit secrets to config files
4. **Implement user restrictions** - Optionally limit access to specific domains:
   ```yaml
   user_restrictions:
     allow_regex: "@yourcompany\\.com$"
   ```
5. **Enable audit logging** - Track all authentication events

---

## Troubleshooting

### Common Issues

| Error | Cause | Solution |
|-------|-------|----------|
| `invalid_grant` | Redirect URI mismatch | Ensure redirect URL exactly matches what's registered |
| `access_denied` | User denied consent | User clicked "Deny" on provider consent screen |
| `invalid_client` | Wrong credentials | Verify client_id and client_secret |
| `invalid_redirect_uri` | Invalid redirect | Check for trailing slashes, protocol match |
| No email in userinfo | Missing scope | Ensure `email` scope is included in provider config |

### Debug Mode

Enable debug logging to see detailed OAuth flow:

```bash
docker run -e LOG_LEVEL=debug ...
```

### Check Provider Endpoints

```bash
# Verify your Entra issuer URL is valid
curl -v "https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0/.well-known/openid-configuration"
```

Expected response includes:
```json
{
  "authorization_endpoint": "https://login.microsoftonline.com/YOUR_TENANT_ID/oauth2/v2.0/authorize",
  "token_endpoint": "https://login.microsoftonline.com/YOUR_TENANT_ID/oauth2/v2.0/token",
  "userinfo_endpoint": "https://graph.microsoft.com/oidc/userinfo"
}
```

---

## Provider Comparison

| Feature | GitHub | Google | Entra ID | Generic OIDC |
|---------|--------|--------|----------|--------------|
| PKCE Support | ✅ | ✅ | ✅ | ✅ |
| Multi-tenant | ❌ | ❌ | ✅ | ⚠️ Provider-dependent |
| Group Claims | ❌ | ⚠️ Add-on | ✅ | ⚠️ Provider-dependent |
| Free Tier | ✅ | ⚠️ Quota | ✅ | Varies |
| Best For | Dev teams | Google Workspace | Azure Orgs | Custom SSO |
