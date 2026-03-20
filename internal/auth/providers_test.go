package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubProviderEndpoints(t *testing.T) {
	p := GitHubProvider()

	if p.Name != "github" {
		t.Errorf("Name = %q, want %q", p.Name, "github")
	}
	if !strings.Contains(p.AuthURL, "github.com/login/oauth/authorize") {
		t.Errorf("AuthURL = %q, should contain github.com/login/oauth/authorize", p.AuthURL)
	}
	if !strings.Contains(p.TokenURL, "github.com/login/oauth/access_token") {
		t.Errorf("TokenURL = %q, should contain github.com/login/oauth/access_token", p.TokenURL)
	}
	if !strings.Contains(p.UserInfoURL, "api.github.com/user") {
		t.Errorf("UserInfoURL = %q, should contain api.github.com/user", p.UserInfoURL)
	}
	if len(p.Scopes) == 0 {
		t.Error("GitHubProvider scopes should not be empty")
	}
	hasReadUser := false
	for _, s := range p.Scopes {
		if s == "read:user" {
			hasReadUser = true
		}
	}
	if !hasReadUser {
		t.Errorf("GitHubProvider scopes should include read:user, got %v", p.Scopes)
	}
}

func TestGoogleProviderEndpoints(t *testing.T) {
	p := GoogleProvider()

	if p.Name != "google" {
		t.Errorf("Name = %q, want %q", p.Name, "google")
	}
	if !strings.Contains(p.AuthURL, "accounts.google.com") {
		t.Errorf("AuthURL = %q, should contain accounts.google.com", p.AuthURL)
	}
	if !strings.Contains(p.TokenURL, "accounts.google.com") {
		t.Errorf("TokenURL = %q, should contain accounts.google.com", p.TokenURL)
	}
	hasOpenID := false
	for _, s := range p.Scopes {
		if s == "openid" {
			hasOpenID = true
		}
	}
	if !hasOpenID {
		t.Errorf("GoogleProvider scopes should include openid, got %v", p.Scopes)
	}
}

func TestOIDCProviderDiscovery(t *testing.T) {
	// Serve a mock OIDC discovery document
	discovery := `{
		"issuer": "https://test.example.com",
		"authorization_endpoint": "https://test.example.com/authorize",
		"token_endpoint": "https://test.example.com/token",
		"userinfo_endpoint": "https://test.example.com/userinfo"
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(discovery))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p, err := OIDCProvider(srv.URL)
	if err != nil {
		t.Fatalf("OIDCProvider() error: %v", err)
	}
	if p.AuthURL != "https://test.example.com/authorize" {
		t.Errorf("AuthURL = %q, want %q", p.AuthURL, "https://test.example.com/authorize")
	}
	if p.TokenURL != "https://test.example.com/token" {
		t.Errorf("TokenURL = %q, want %q", p.TokenURL, "https://test.example.com/token")
	}
}

func TestOIDCProviderDiscoveryFailure(t *testing.T) {
	_, err := OIDCProvider("http://localhost:0") // unreachable
	if err == nil {
		t.Error("OIDCProvider() should return error when discovery fails")
	}
}
