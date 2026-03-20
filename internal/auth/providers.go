package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// OAuthProvider holds the OAuth 2.1 / OIDC endpoint configuration for a provider.
type OAuthProvider struct {
	// Name is the provider identifier (e.g. "github", "google", "oidc").
	Name string
	// AuthURL is the provider's authorization endpoint.
	AuthURL string
	// TokenURL is the provider's token exchange endpoint.
	TokenURL string
	// UserInfoURL is the endpoint to validate tokens and retrieve user identity.
	UserInfoURL string
	// Scopes is the list of OAuth scopes to request during authorization.
	Scopes []string
}

// GitHubProvider returns the OAuth endpoint configuration for GitHub.
// Scopes: read:user, user:email.
func GitHubProvider() OAuthProvider {
	return OAuthProvider{
		Name:        "github",
		AuthURL:     "https://github.com/login/oauth/authorize",
		TokenURL:    "https://github.com/login/oauth/access_token",
		UserInfoURL: "https://api.github.com/user",
		Scopes:      []string{"read:user", "user:email"},
	}
}

// GoogleProvider returns the OIDC endpoint configuration for Google.
// Scopes: openid, email, profile.
func GoogleProvider() OAuthProvider {
	return OAuthProvider{
		Name:        "google",
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
		Scopes:      []string{"openid", "email", "profile"},
	}
}

// oidcDiscovery is the subset of an OIDC discovery document we need.
type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

// OIDCProvider discovers endpoints from the issuer's .well-known/openid-configuration.
// Returns an error if the discovery document cannot be fetched or parsed.
func OIDCProvider(issuerURL string) (OAuthProvider, error) {
	discoveryURL := strings.TrimSuffix(issuerURL, "/") + "/.well-known/openid-configuration"
	resp, err := http.Get(discoveryURL) //nolint:noctx // discovery is a one-time startup call
	if err != nil {
		return OAuthProvider{}, fmt.Errorf("fetching OIDC discovery document from %q: %w", discoveryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OAuthProvider{}, fmt.Errorf("OIDC discovery returned HTTP %d from %q", resp.StatusCode, discoveryURL)
	}

	var doc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return OAuthProvider{}, fmt.Errorf("parsing OIDC discovery document: %w", err)
	}

	if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return OAuthProvider{}, fmt.Errorf("OIDC discovery document missing required endpoints")
	}

	return OAuthProvider{
		Name:        "oidc",
		AuthURL:     doc.AuthorizationEndpoint,
		TokenURL:    doc.TokenEndpoint,
		UserInfoURL: doc.UserInfoEndpoint,
		Scopes:      []string{"openid", "email", "profile"},
	}, nil
}
