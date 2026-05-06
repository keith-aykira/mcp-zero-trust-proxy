package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
)

// mockUserInfoServer creates a test server that validates Bearer tokens and returns user info.
func mockUserInfoServer(validToken, userID, email string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+validToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    userID,
			"login": userID,
			"email": email,
		})
	}))
}

// mockTokenServer creates a test server that handles code+PKCE exchange.
func mockTokenServer(validCode, validVerifier, expectedChallenge, returnToken string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		verifier := r.FormValue("code_verifier")

		if code != validCode {
			http.Error(w, "invalid_grant", http.StatusBadRequest)
			return
		}
		// Verify PKCE
		if !VerifyCodeChallenge(verifier, expectedChallenge) {
			http.Error(w, "invalid_grant: pkce mismatch", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": returnToken,
			"token_type":   "bearer",
		})
	}))
}

func TestAuthenticateValidToken(t *testing.T) {
	const validToken = "valid-token-abc123"
	const userID = "user42"
	const email = "user42@example.com"

	userInfoSrv := mockUserInfoServer(validToken, userID, email)
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{
		Provider: "github",
		ClientID: "test-client-id",
	}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	// Override the userinfo URL to point to mock server
	auth.provider.UserInfoURL = userInfoSrv.URL

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	identity, err := auth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() returned unexpected error: %v", err)
	}
	if identity.ClientID != userID {
		t.Errorf("ClientID = %q, want %q", identity.ClientID, userID)
	}
	if identity.Email != email {
		t.Errorf("Email = %q, want %q", identity.Email, email)
	}
	if identity.SessionID == "" {
		t.Error("SessionID should not be empty after authentication")
	}
}

func TestAuthenticateNoAuthHeader(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	req := httptest.NewRequest("POST", "/", nil)
	// No Authorization header

	_, authErr := auth.Authenticate(req)
	if authErr == nil {
		t.Error("Authenticate() should return error when Authorization header is absent")
	}
}

func TestAuthenticateMalformedToken(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	tests := []struct {
		name   string
		header string
	}{
		{"empty bearer", "Bearer "},
		{"no bearer prefix", "token-without-bearer"},
		{"basic auth", "Basic dXNlcjpwYXNz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", nil)
			req.Header.Set("Authorization", tt.header)
			_, authErr := auth.Authenticate(req)
			if authErr == nil {
				t.Errorf("Authenticate() should return error for malformed header %q", tt.header)
			}
		})
	}
}

func TestAuthenticateInvalidToken(t *testing.T) {
	// UserInfo server rejects bad tokens with 401
	userInfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	auth.provider.UserInfoURL = userInfoSrv.URL

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	_, authErr := auth.Authenticate(req)
	if authErr == nil {
		t.Error("Authenticate() should return error for token rejected by provider")
	}
}

func TestTokenCache(t *testing.T) {
	callCount := 0
	userInfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "user1",
			"login": "user1",
			"email": "user1@example.com",
		})
	}))
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	auth.provider.UserInfoURL = userInfoSrv.URL

	// Same token, two requests — should hit userinfo only once
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Authorization", "Bearer cached-token")
		_, _ = auth.Authenticate(req)
	}

	if callCount > 1 {
		t.Errorf("userinfo endpoint called %d times, expected at most 1 (token should be cached)", callCount)
	}
}

func TestHandleAuthStart(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{
		Provider:    "github",
		ClientID:    "test-client-id",
		RedirectURL: "http://localhost:8080/callback",
	}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	req := httptest.NewRequest("GET", "/auth/start", nil)
	rr := httptest.NewRecorder()

	auth.HandleAuthStart(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusFound)
	}

	location := rr.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("Location URL is invalid: %v", err)
	}

	q := u.Query()
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", q.Get("code_challenge_method"))
	}
	if q.Get("code_challenge") == "" {
		t.Error("code_challenge is missing from redirect URL")
	}
	if q.Get("state") == "" {
		t.Error("state is missing from redirect URL")
	}
	if q.Get("client_id") != "test-client-id" {
		t.Errorf("client_id = %q, want test-client-id", q.Get("client_id"))
	}

	// Verify CSRF cookie is set with the same state value
	state := q.Get("state")
	cookies := rr.Result().Cookies()
	var oauthStateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_state" {
			oauthStateCookie = c
			break
		}
	}
	if oauthStateCookie == nil {
		t.Fatal("oauth_state cookie not set in HandleAuthStart response")
	}
	if oauthStateCookie.Value != state {
		t.Errorf("oauth_state cookie value = %q, want %q (state from redirect URL)", oauthStateCookie.Value, state)
	}
	if !oauthStateCookie.HttpOnly {
		t.Error("oauth_state cookie must be HttpOnly")
	}
	if oauthStateCookie.MaxAge != 600 {
		t.Errorf("oauth_state cookie MaxAge = %d, want 600", oauthStateCookie.MaxAge)
	}
}

func TestHandleCallbackPKCESuccess(t *testing.T) {
	const returnToken = "exchange-result-token"

	// First, generate a code verifier and challenge
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier() error: %v", err)
	}
	challenge := GenerateCodeChallenge(verifier)

	tokenSrv := mockTokenServer("auth-code-123", verifier, challenge, returnToken)
	defer tokenSrv.Close()

	userInfoSrv := mockUserInfoServer(returnToken, "user1", "user1@example.com")
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{
		Provider:    "github",
		ClientID:    "test-client-id",
		RedirectURL: "http://localhost:8080/callback",
	}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	auth.provider.TokenURL = tokenSrv.URL
	auth.provider.UserInfoURL = userInfoSrv.URL

	// Manually inject the PKCE state so callback can retrieve it
	state := "test-state-value"
	auth.stateCache.Store(state, pkceEntry{
		verifier:  verifier,
		createdAt: time.Now(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/callback?code=auth-code-123&state=%s", state), nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	rr := httptest.NewRecorder()

	auth.HandleCallback(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("callback status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	// Response should contain the access token
	body := rr.Body.String()
	if !strings.Contains(body, returnToken) {
		t.Errorf("callback response should contain access token %q; got: %s", returnToken, body)
	}
}

func TestHandleCallbackPKCEWrongVerifier(t *testing.T) {
	wrongVerifier, _ := GenerateCodeVerifier()
	correctVerifier, _ := GenerateCodeVerifier()
	challenge := GenerateCodeChallenge(correctVerifier)

	tokenSrv := mockTokenServer("auth-code-456", correctVerifier, challenge, "token")
	defer tokenSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	auth, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	auth.provider.TokenURL = tokenSrv.URL

	// Inject wrong verifier into state cache
	state := "state-wrong"
	auth.stateCache.Store(state, pkceEntry{
		verifier:  wrongVerifier,
		createdAt: time.Now(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/callback?code=auth-code-456&state=%s", state), nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	rr := httptest.NewRecorder()

	auth.HandleCallback(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("callback should not succeed when PKCE verifier is wrong")
	}
}

// ========================
// NEW TESTS: Cache cleanup, client secret, error sanitization
// ========================

func TestStartCleanupRemovesExpiredTokenCacheEntries(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	// Store an already-expired token cache entry
	a.tokenCache.Store("expired-token", tokenCacheEntry{
		identity:  nil,
		expiresAt: time.Now().Add(-1 * time.Minute), // expired
	})
	// Store a valid (non-expired) entry
	a.tokenCache.Store("valid-token", tokenCacheEntry{
		identity:  nil,
		expiresAt: time.Now().Add(5 * time.Minute), // not expired
	})

	// Run cleanup manually (tick once)
	a.runCleanup()

	// Expired entry should be gone
	if _, ok := a.tokenCache.Load("expired-token"); ok {
		t.Error("expired token cache entry should have been removed by cleanup")
	}
	// Valid entry should remain
	if _, ok := a.tokenCache.Load("valid-token"); !ok {
		t.Error("valid token cache entry should NOT have been removed by cleanup")
	}
}

func TestStartCleanupRemovesExpiredStateCacheEntries(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	// Store an expired state entry (created >10 minutes ago)
	a.stateCache.Store("old-state", pkceEntry{
		verifier:  "verifier1",
		createdAt: time.Now().Add(-11 * time.Minute), // older than 10 min
	})
	// Store a fresh state entry
	a.stateCache.Store("fresh-state", pkceEntry{
		verifier:  "verifier2",
		createdAt: time.Now(), // just created
	})

	// Run cleanup manually
	a.runCleanup()

	// Old state should be gone
	if _, ok := a.stateCache.Load("old-state"); ok {
		t.Error("expired state cache entry (>10 min old) should have been removed by cleanup")
	}
	// Fresh state should remain
	if _, ok := a.stateCache.Load("fresh-state"); !ok {
		t.Error("fresh state cache entry should NOT have been removed by cleanup")
	}
}

func TestStartStopCleanup(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	// StartCleanup should not panic and should start a goroutine
	a.StartCleanup()

	// StopCleanup should not panic and should stop the goroutine cleanly
	// Call it twice to ensure idempotency (no double-close panic)
	a.StopCleanup()
}

func TestExchangeCodeIncludesClientSecretWhenConfigured(t *testing.T) {
	const clientSecret = "super-secret-value"
	var receivedSecret string

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		receivedSecret = r.FormValue("client_secret")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "tok-123",
			"token_type":   "bearer",
		})
	}))
	defer tokenSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{
		Provider:     "github",
		ClientID:     "test-client-id",
		ClientSecret: clientSecret,
	}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.TokenURL = tokenSrv.URL

	_, err = a.exchangeCode("some-code", "some-verifier")
	if err != nil {
		t.Fatalf("exchangeCode() unexpected error: %v", err)
	}

	if receivedSecret != clientSecret {
		t.Errorf("client_secret sent = %q, want %q", receivedSecret, clientSecret)
	}
}

func TestExchangeCodeOmitsClientSecretWhenEmpty(t *testing.T) {
	var receivedSecret string

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		receivedSecret = r.FormValue("client_secret")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "tok-456",
			"token_type":   "bearer",
		})
	}))
	defer tokenSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{
		Provider:     "github",
		ClientID:     "test-client-id",
		ClientSecret: "", // empty — should not be sent
	}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.TokenURL = tokenSrv.URL

	_, err = a.exchangeCode("some-code", "some-verifier")
	if err != nil {
		t.Fatalf("exchangeCode() unexpected error: %v", err)
	}

	if receivedSecret != "" {
		t.Errorf("client_secret should not be sent when empty, but got %q", receivedSecret)
	}
}

func TestExchangeCodeErrorDoesNotLeakProviderBody(t *testing.T) {
	const sensitiveBody = "internal_error: secret provider details xyz"

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, sensitiveBody, http.StatusBadRequest)
	}))
	defer tokenSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.TokenURL = tokenSrv.URL

	_, err = a.exchangeCode("bad-code", "bad-verifier")
	if err == nil {
		t.Fatal("exchangeCode() should return error on non-200 response")
	}

	// The error message must NOT contain the raw provider response body
	if strings.Contains(err.Error(), sensitiveBody) {
		t.Errorf("exchangeCode error leaked provider body: %q", err.Error())
	}
	// Must NOT contain "internal_error" or "secret provider"
	if strings.Contains(err.Error(), "internal_error") {
		t.Errorf("exchangeCode error contains provider-specific content: %q", err.Error())
	}
}

func TestAuthenticateErrorIsSanitized(t *testing.T) {
	// Provider returns 401 with sensitive details
	const sensitiveMsg = "oauth2: token_rejected reason=abuse_detected user=hacker"
	userInfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, sensitiveMsg, http.StatusUnauthorized)
	}))
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.UserInfoURL = userInfoSrv.URL

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")

	_, authErr := a.Authenticate(req)
	if authErr == nil {
		t.Fatal("Authenticate() should return error for rejected token")
	}

	// Error returned to caller must not contain provider-specific sensitive details
	if strings.Contains(authErr.Error(), sensitiveMsg) {
		t.Errorf("Authenticate error leaked sensitive provider info: %q", authErr.Error())
	}
	if strings.Contains(authErr.Error(), "token_rejected") {
		t.Errorf("Authenticate error contains provider-specific content: %q", authErr.Error())
	}
}

// TestResolveRole_MappedEmail verifies that a known email resolves to its mapped role.
func TestResolveRole_MappedEmail(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	a.SetUserRoles(map[string]string{
		"alice@example.com": "admin",
		"bob@example.com":   "restricted",
	}, "readonly")

	if role := a.resolveRole("alice@example.com"); role != "admin" {
		t.Errorf("resolveRole(alice) = %q, want %q", role, "admin")
	}
	if role := a.resolveRole("bob@example.com"); role != "restricted" {
		t.Errorf("resolveRole(bob) = %q, want %q", role, "restricted")
	}
}

// TestResolveRole_UnmappedEmail verifies that an unknown email returns the default role.
func TestResolveRole_UnmappedEmail(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	a.SetUserRoles(map[string]string{
		"alice@example.com": "admin",
	}, "restricted")

	if role := a.resolveRole("unknown@example.com"); role != "restricted" {
		t.Errorf("resolveRole(unknown) = %q, want %q", role, "restricted")
	}
}

// TestResolveRole_DefaultWhenNoMapping verifies the default role is used when no mapping is set.
func TestResolveRole_DefaultWhenNoMapping(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	// No SetUserRoles call — should use hardcoded default ("readonly")
	if role := a.resolveRole("anyone@example.com"); role != "readonly" {
		t.Errorf("resolveRole without mapping = %q, want %q", role, "readonly")
	}
}

// TestAuthenticateUsesResolvedRole verifies that fetched identity uses resolveRole result.
func TestAuthenticateUsesResolvedRole(t *testing.T) {
	const validToken = "valid-token-role-test"
	const adminEmail = "admin@example.com"

	userInfoSrv := mockUserInfoServer(validToken, "user1", adminEmail)
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.UserInfoURL = userInfoSrv.URL
	a.SetUserRoles(map[string]string{
		adminEmail: "admin",
	}, "readonly")

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	identity, err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}

	if identity.Role != "admin" {
		t.Errorf("identity.Role = %q, want %q", identity.Role, "admin")
	}
}

// TestClaimRuleEvaluation_EqualOperator verifies equals operator matching.
func TestClaimRuleEvaluation_EqualOperator(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	rules := []config.ClaimRule{
		{Claim: "department", Operator: "equals", Value: "engineering", Role: "admin"},
		{Claim: "department", Operator: "equals", Value: "sales", Role: "readonly"},
	}
	a.SetClaimRules(rules)

	tests := []struct {
		name   string
		claims map[string]interface{}
		want   string
	}{
		{"Engineering dept", map[string]interface{}{"department": "engineering"}, "admin"},
		{"Sales dept", map[string]interface{}{"department": "sales"}, "readonly"},
		{"Other dept", map[string]interface{}{"department": "marketing"}, ""},
		{"Missing claim", map[string]interface{}{"name": "alice"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.evaluateClaimRules(tt.claims)
			if got != tt.want {
				t.Errorf("evaluateClaimRules(%v) = %q, want %q", tt.claims, got, tt.want)
			}
		})
	}
}

// TestClaimRuleEvaluation_ContainsOperator verifies contains operator matching.
func TestClaimRuleEvaluation_ContainsOperator(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	rules := []config.ClaimRule{
		{Claim: "groups", Operator: "contains", Value: "admins", Role: "admin"},
		{Claim: "groups", Operator: "contains", Value: "developers", Role: "readonly"},
	}
	a.SetClaimRules(rules)

	tests := []struct {
		name   string
		claims map[string]interface{}
		want   string
	}{
		{"Contains admins", map[string]interface{}{"groups": "super-admins-team"}, "admin"},
		{"Contains developers", map[string]interface{}{"groups": "backend-developers"}, "readonly"},
		{"Neither match", map[string]interface{}{"groups": "visitors"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.evaluateClaimRules(tt.claims)
			if got != tt.want {
				t.Errorf("evaluateClaimRules(%v) = %q, want %q", tt.claims, got, tt.want)
			}
		})
	}
}

// TestClaimRuleEvaluation_StartsWithOperator verifies starts_with operator matching.
func TestClaimRuleEvaluation_StartsWithOperator(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	rules := []config.ClaimRule{
		{Claim: "email", Operator: "starts_with", Value: "admin@", Role: "admin"},
		{Claim: "email", Operator: "starts_with", Value: "dev@", Role: "readonly"},
	}
	a.SetClaimRules(rules)

	tests := []struct {
		name   string
		claims map[string]interface{}
		want   string
	}{
		{"Admin email", map[string]interface{}{"email": "admin@company.com"}, "admin"},
		{"Dev email", map[string]interface{}{"email": "dev@company.com"}, "readonly"},
		{"Other email", map[string]interface{}{"email": "user@company.com"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.evaluateClaimRules(tt.claims)
			if got != tt.want {
				t.Errorf("evaluateClaimRules(%v) = %q, want %q", tt.claims, got, tt.want)
			}
		})
	}
}

// TestClaimRuleEvaluation_EndsWithOperator verifies ends_with operator matching.
func TestClaimRuleEvaluation_EndsWithOperator(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	rules := []config.ClaimRule{
		{Claim: "email", Operator: "ends_with", Value: ".admin.com", Role: "admin"},
		{Claim: "email", Operator: "ends_with", Value: ".corp.com", Role: "readonly"},
	}
	a.SetClaimRules(rules)

	tests := []struct {
		name   string
		claims map[string]interface{}
		want   string
	}{
		{"Admin domain", map[string]interface{}{"email": "user@company.admin.com"}, "admin"},
		{"Corp domain", map[string]interface{}{"email": "user@company.corp.com"}, "readonly"},
		{"Other domain", map[string]interface{}{"email": "user@external.com"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.evaluateClaimRules(tt.claims)
			if got != tt.want {
				t.Errorf("evaluateClaimRules(%v) = %q, want %q", tt.claims, got, tt.want)
			}
		})
	}
}

// TestClaimRuleEvaluation_RegexOperator verifies regex operator matching.
func TestClaimRuleEvaluation_RegexOperator(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	rules := []config.ClaimRule{
		{Claim: "email", Operator: "regex", Value: `^svc-.*@internal\.com$`, Role: "admin"},
		{Claim: "email", Operator: "regex", Value: `^user.*@company\.com$`, Role: "readonly"},
	}
	a.SetClaimRules(rules)

	tests := []struct {
		name   string
		claims map[string]interface{}
		want   string
	}{
		{"Service account", map[string]interface{}{"email": "svc-backend@internal.com"}, "admin"},
		{"Regular user", map[string]interface{}{"email": "user123@company.com"}, "readonly"},
		{"Non-matching", map[string]interface{}{"email": "guest@external.com"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.evaluateClaimRules(tt.claims)
			if got != tt.want {
				t.Errorf("evaluateClaimRules(%v) = %q, want %q", tt.claims, got, tt.want)
			}
		})
	}
}

// TestClaimRuleEvaluation_FirstMatchWins verifies that the first matching rule wins.
func TestClaimRuleEvaluation_FirstMatchWins(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}

	rules := []config.ClaimRule{
		{Claim: "role", Operator: "equals", Value: "power", Role: "restricted"},
		{Claim: "role", Operator: "equals", Value: "power", Role: "admin"},
	}
	a.SetClaimRules(rules)

	// Both rules match, but first one should win
	claims := map[string]interface{}{"role": "power"}
	got := a.evaluateClaimRules(claims)
	if got != "restricted" {
		t.Errorf("evaluateClaimRules should return first match 'restricted', got %q", got)
	}
}

// TestAuthenticateWithClaims verifies that claims are extracted and used for role resolution.
func TestAuthenticateWithClaims(t *testing.T) {
	const validToken = "valid-token-claims-test"
	const email = "alice@example.com"

	userInfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+validToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         "user1",
			"login":      "alice",
			"email":      email,
			"department": "engineering",
			"groups":     "admin-group",
		})
	}))
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.UserInfoURL = userInfoSrv.URL

	// Configure claim rule: department = engineering -> admin
	rules := []config.ClaimRule{
		{Claim: "department", Operator: "equals", Value: "engineering", Role: "admin"},
	}
	a.SetClaimRules(rules)

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	identity, err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}

	if identity.Role != "admin" {
		t.Errorf("identity.Role = %q, want %q (from claim rule)", identity.Role, "admin")
	}

	if identity.Claims == nil || identity.Claims["department"] != "engineering" {
		t.Errorf("identity.Claims not properly set: %v", identity.Claims)
	}
}

// TestAuthenticateClaimsFallbackToEmail verifies that when no claim rule matches,
// email-based role mapping is used as fallback.
func TestAuthenticateClaimsFallbackToEmail(t *testing.T) {
	const validToken = "valid-token-fallback-test"
	const email = "bob@example.com"

	userInfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+validToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "user2",
			"login": "bob",
			"email": email,
		})
	}))
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.UserInfoURL = userInfoSrv.URL

	// Configure claim rule that won't match (wrong department)
	rules := []config.ClaimRule{
		{Claim: "department", Operator: "equals", Value: "engineering", Role: "admin"},
	}
	a.SetClaimRules(rules)

	// Configure email mapping for fallback
	a.SetUserRoles(map[string]string{
		email: "readonly",
	}, "restricted")

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	identity, err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}

	if identity.Role != "readonly" {
		t.Errorf("identity.Role = %q, want %q (fallback to email mapping)", identity.Role, "readonly")
	}
}

// TestAuthenticateClaimsFallbackToDefault verifies that when no claim rule matches
// and no email mapping exists, the default role is used.
func TestAuthenticateClaimsFallbackToDefault(t *testing.T) {
	const validToken = "valid-token-default-test"
	const email = "unknown@example.com"

	userInfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+validToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "user3",
			"login": "unknown",
			"email": email,
		})
	}))
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.UserInfoURL = userInfoSrv.URL

	// Configure claim rule that won't match (wrong department)
	rules := []config.ClaimRule{
		{Claim: "department", Operator: "equals", Value: "engineering", Role: "admin"},
	}
	a.SetClaimRules(rules)

	// No email mapping for this user, only default
	a.SetUserRoles(map[string]string{}, "restricted")

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	identity, err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}

	if identity.Role != "restricted" {
		t.Errorf("identity.Role = %q, want %q (fallback to default)", identity.Role, "restricted")
	}
}

func TestHandleCallbackErrorIsSanitized(t *testing.T) {
	// Token endpoint returns error with sensitive provider details
	const sensitiveBody = "error=invalid_grant&error_description=secret_rotation_policy"
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, sensitiveBody, http.StatusBadRequest)
	}))
	defer tokenSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{Provider: "github", ClientID: "test-client-id"}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.TokenURL = tokenSrv.URL

	// Inject a valid state entry
	state := "callback-state"
	verifier, _ := GenerateCodeVerifier()
	a.stateCache.Store(state, pkceEntry{
		verifier:  verifier,
		createdAt: time.Now(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/callback?code=bad-code&state=%s", state), nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	rr := httptest.NewRecorder()

	a.HandleCallback(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("HandleCallback should fail when token exchange fails")
	}

	// Response body must not contain the raw provider error
	body := rr.Body.String()
	if strings.Contains(body, sensitiveBody) {
		t.Errorf("HandleCallback response leaked sensitive provider body: %q", body)
	}
	if strings.Contains(body, "secret_rotation_policy") {
		t.Errorf("HandleCallback response contains provider-specific content: %q", body)
	}
}

// TestHandleCallbackCSRFProtection verifies that the callback rejects requests
// when the oauth_state cookie is absent or does not match the state query parameter.
func TestHandleCallbackCSRFProtection(t *testing.T) {
	const returnToken = "csrf-test-token"

	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier() error: %v", err)
	}
	challenge := GenerateCodeChallenge(verifier)

	tokenSrv := mockTokenServer("auth-code-csrf", verifier, challenge, returnToken)
	defer tokenSrv.Close()

	userInfoSrv := mockUserInfoServer(returnToken, "user1", "user1@example.com")
	defer userInfoSrv.Close()

	store := NewSessionStore(1 * time.Hour)
	defer store.Stop()

	cfg := &config.AuthConfig{
		Provider:    "github",
		ClientID:    "test-client-id",
		RedirectURL: "http://localhost:8080/callback",
	}
	a, err := NewAuthenticator(cfg, store)
	if err != nil {
		t.Fatalf("NewAuthenticator() error: %v", err)
	}
	a.provider.TokenURL = tokenSrv.URL
	a.provider.UserInfoURL = userInfoSrv.URL

	const state = "csrf-test-state"

	setupState := func() {
		a.stateCache.Store(state, pkceEntry{
			verifier:  verifier,
			createdAt: time.Now(),
		})
	}

	t.Run("no cookie → 403", func(t *testing.T) {
		setupState()
		req := httptest.NewRequest("GET", fmt.Sprintf("/callback?code=auth-code-csrf&state=%s", state), nil)
		// No cookie set — simulates an attacker-crafted redirect to the victim
		rr := httptest.NewRecorder()
		a.HandleCallback(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d (CSRF: no cookie)", rr.Code, http.StatusForbidden)
		}
		if !strings.Contains(rr.Body.String(), "CSRF") {
			t.Errorf("expected CSRF error message, got: %s", rr.Body.String())
		}
	})

	t.Run("wrong cookie value → 403", func(t *testing.T) {
		setupState()
		req := httptest.NewRequest("GET", fmt.Sprintf("/callback?code=auth-code-csrf&state=%s", state), nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "different-state-value"})
		rr := httptest.NewRecorder()
		a.HandleCallback(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d (CSRF: cookie mismatch)", rr.Code, http.StatusForbidden)
		}
		if !strings.Contains(rr.Body.String(), "CSRF") {
			t.Errorf("expected CSRF error message, got: %s", rr.Body.String())
		}
	})

	t.Run("correct cookie → 200", func(t *testing.T) {
		setupState()
		req := httptest.NewRequest("GET", fmt.Sprintf("/callback?code=auth-code-csrf&state=%s", state), nil)
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
		rr := httptest.NewRecorder()
		a.HandleCallback(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d (valid CSRF cookie); body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), returnToken) {
			t.Errorf("response should contain access token %q; got: %s", returnToken, rr.Body.String())
		}
	})
}
