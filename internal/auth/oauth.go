package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
	"github.com/rs/zerolog/log"
)

// pkceEntry holds a PKCE code verifier and its creation time for expiry enforcement.
type pkceEntry struct {
	verifier  string
	createdAt time.Time
}

// tokenCacheEntry caches a validated token's identity to avoid per-request userinfo calls.
type tokenCacheEntry struct {
	identity  *proxy.ClientIdentity
	expiresAt time.Time
}

// Authenticator implements OAuth 2.1 PKCE token validation, auth flow start, and callback handling.
// It is safe for concurrent use.
type Authenticator struct {
	cfg          *config.AuthConfig
	provider     OAuthProvider
	sessionStore *SessionStore
	httpClient   *http.Client

	// stateCache maps state parameter -> pkceEntry. Entries expire after 10 minutes.
	stateCache sync.Map

	// tokenCache maps Bearer token -> tokenCacheEntry. Valid tokens are cached for 5 minutes.
	tokenCache sync.Map

	// stopCleanup is closed to signal the background cleanup goroutine to stop.
	stopCleanup chan struct{}

	// userRoles maps email -> role name for role resolution.
	userRoles   map[string]string
	defaultRole string
	claimRules  []config.ClaimRule
	rolesMu     sync.RWMutex

	// user restrictions: allow/deny regex patterns
	allowRegex *regexp.Regexp
	denyRegex  *regexp.Regexp
	restrictMu sync.RWMutex
}

// NewAuthenticator constructs an Authenticator from the given AuthConfig and SessionStore.
// It selects the OAuth provider by cfg.Provider name.
// For OIDC providers, it performs a one-time discovery request.
func NewAuthenticator(cfg *config.AuthConfig, sessionStore *SessionStore) (*Authenticator, error) {
	var provider OAuthProvider
	var err error

	switch cfg.Provider {
	case "github", "":
		provider = GitHubProvider()
	case "google":
		provider = GoogleProvider()
	case "oidc":
		provider, err = OIDCProvider(cfg.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("OIDC discovery failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown provider %q: valid values are github, google, oidc", cfg.Provider)
	}

	return &Authenticator{
		cfg:          cfg,
		provider:     provider,
		sessionStore: sessionStore,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		stopCleanup:  make(chan struct{}),
	}, nil
}

// StartCleanup launches a background goroutine that periodically removes expired
// entries from stateCache (>10 minutes old) and tokenCache (past expiresAt).
// It runs every 60 seconds until StopCleanup is called.
func (a *Authenticator) StartCleanup() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.runCleanup()
			case <-a.stopCleanup:
				return
			}
		}
	}()
}

// StopCleanup signals the background cleanup goroutine to stop.
// Safe to call multiple times.
func (a *Authenticator) StopCleanup() {
	select {
	case <-a.stopCleanup:
		// already closed, nothing to do
	default:
		close(a.stopCleanup)
	}
}

// runCleanup performs a single pass of cache expiry cleanup.
// Called periodically by the goroutine started by StartCleanup, and directly in tests.
func (a *Authenticator) runCleanup() {
	now := time.Now()

	// Remove expired token cache entries (past expiresAt)
	a.tokenCache.Range(func(key, val interface{}) bool {
		entry := val.(tokenCacheEntry)
		if now.After(entry.expiresAt) {
			a.tokenCache.Delete(key)
		}
		return true
	})

	// Remove expired state cache entries (created more than 10 minutes ago)
	a.stateCache.Range(func(key, val interface{}) bool {
		entry := val.(pkceEntry)
		if now.Sub(entry.createdAt) > 10*time.Minute {
			a.stateCache.Delete(key)
		}
		return true
	})
}

// SetUserRoles configures email-to-role mapping and a default role for unrecognized emails.
// This must be called before the first Authenticate call if role mapping is desired.
// It is safe to call from multiple goroutines.
func (a *Authenticator) SetUserRoles(mapping map[string]string, defaultRole string) {
	a.rolesMu.Lock()
	defer a.rolesMu.Unlock()
	a.userRoles = mapping
	a.defaultRole = defaultRole
}

// SetClaimRules configures claim-based role mapping rules.
// This must be called before the first Authenticate call if claim-based role mapping is desired.
// It is safe to call from multiple goroutines.
func (a *Authenticator) SetClaimRules(rules []config.ClaimRule) {
	a.rolesMu.Lock()
	defer a.rolesMu.Unlock()
	a.claimRules = rules
}

// SetUserRestrictions configures the allow/deny regex patterns for user access control.
// This must be called before the first Authenticate call if user restrictions are desired.
// It is safe to call from multiple goroutines.
func (a *Authenticator) SetUserRestrictions(allowRegex, denyRegex string) error {
	a.restrictMu.Lock()
	defer a.restrictMu.Unlock()

	if allowRegex != "" {
		regex, err := regexp.Compile(allowRegex)
		if err != nil {
			return fmt.Errorf("compiling allow regex %q: %w", allowRegex, err)
		}
		a.allowRegex = regex
	} else {
		a.allowRegex = nil
	}

	if denyRegex != "" {
		regex, err := regexp.Compile(denyRegex)
		if err != nil {
			return fmt.Errorf("compiling deny regex %q: %w", denyRegex, err)
		}
		a.denyRegex = regex
	} else {
		a.denyRegex = nil
	}

	return nil
}

// isDenied returns true if the email matches the deny regex (user is explicitly blocked).
func (a *Authenticator) isDenied(email string) bool {
	a.restrictMu.RLock()
	defer a.restrictMu.RUnlock()
	if a.denyRegex != nil {
		return a.denyRegex.MatchString(email)
	}
	return false
}

// isAllowed returns true if the allow regex is not set or the email matches it.
func (a *Authenticator) isAllowed(email string) bool {
	a.restrictMu.RLock()
	defer a.restrictMu.RUnlock()
	if a.allowRegex == nil {
		return true // no allow restriction means everyone is allowed
	}
	return a.allowRegex.MatchString(email)
}

// resolveRole returns the RBAC role for the given email address.
// If a mapping exists and the email is found, returns the mapped role.
// Otherwise returns the configured default role (or "readonly" if none set).
func (a *Authenticator) resolveRole(email string) string {
	a.rolesMu.RLock()
	defer a.rolesMu.RUnlock()
	if a.userRoles != nil {
		if role, ok := a.userRoles[email]; ok {
			return role
		}
	}
	if a.defaultRole != "" {
		return a.defaultRole
	}
	return "readonly"
}

// evaluateClaimRules checks the claim rules against the provided claims.
// Returns the role from the first matching rule, or empty string if no rule matches.
func (a *Authenticator) evaluateClaimRules(claims map[string]interface{}) string {
	for _, rule := range a.claimRules {
		claimValue, exists := claims[rule.Claim]
		if !exists {
			continue
		}
		claimStr, ok := claimValue.(string)
		if !ok {
			log.Debug().Str("claim", rule.Claim).Interface("value", claimValue).
				Msg("Claim rule skipped: non-string claim value")
			continue
		}
		matched := false
		switch rule.Operator {
		case "equals":
			matched = claimStr == rule.Value
		case "contains":
			matched = strings.Contains(claimStr, rule.Value)
		case "starts_with":
			matched = strings.HasPrefix(claimStr, rule.Value)
		case "ends_with":
			matched = strings.HasSuffix(claimStr, rule.Value)
		case "regex":
			matched, _ = regexp.MatchString(rule.Value, claimStr)
		}
		if matched {
			return rule.Role
		}
	}
	return ""
}

// Authenticate validates the Bearer token in the request's Authorization header.
// On success it returns a ClientIdentity with ClientID, Email, Role, and SessionID.
// On failure it returns a non-nil error — the caller should respond with 401.
// Error messages are sanitized — no provider-internal details are returned to callers.
func (a *Authenticator) Authenticate(r *http.Request) (*proxy.ClientIdentity, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}

	// Check token cache first (5-minute TTL)
	if cached, ok := a.tokenCache.Load(token); ok {
		entry := cached.(tokenCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.identity, nil
		}
		a.tokenCache.Delete(token)
	}

	// Call provider's userinfo endpoint to validate token and get identity.
	// Internal error details are not propagated to the caller.
	identity, err := a.fetchUserIdentity(token)
	if err != nil {
		return nil, fmt.Errorf("authentication failed")
	}

	// Check user restrictions: deny-list first (takes precedence), then allow-list
	if a.isDenied(identity.Email) {
		return nil, fmt.Errorf("access denied: user is restricted")
	}
	if !a.isAllowed(identity.Email) {
		return nil, fmt.Errorf("user not allowed")
	}

	// Apply role resolution: first try claim-based rules, then fall back to email mapping.
	claimRole := a.evaluateClaimRules(identity.Claims)
	if claimRole != "" {
		identity.Role = claimRole
	} else {
		identity.Role = a.resolveRole(identity.Email)
	}

	// Create or reuse a session for this identity
	session, err := a.sessionStore.Create(identity)
	if err != nil {
		return nil, fmt.Errorf("session creation failed: %w", err)
	}
	identity.SessionID = session.ID

	// Cache the validated identity
	a.tokenCache.Store(token, tokenCacheEntry{
		identity:  identity,
		expiresAt: time.Now().Add(5 * time.Minute),
	})

	return identity, nil
}

// HandleAuthStart generates a PKCE verifier/challenge, stores it in the state cache,
// and redirects the client to the provider's authorization URL.
func (a *Authenticator) HandleAuthStart(w http.ResponseWriter, r *http.Request) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		http.Error(w, "failed to generate PKCE verifier", http.StatusInternalServerError)
		return
	}
	challenge := GenerateCodeChallenge(verifier)

	// Generate a random state parameter (16 random bytes → 22 base64url chars)
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Store verifier under state key (10-minute expiry enforced on retrieval)
	a.stateCache.Store(state, pkceEntry{
		verifier:  verifier,
		createdAt: time.Now(),
	})

	// Bind the state to the user's browser session via an HTTP-only cookie.
	// This prevents OAuth login CSRF: an attacker who obtains a redirect URL
	// with their own state parameter cannot trick a victim into completing the
	// flow, because the victim's browser will not have the matching cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	// Build authorization URL
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", a.cfg.ClientID)
	if a.cfg.RedirectURL != "" {
		params.Set("redirect_uri", a.cfg.RedirectURL)
	}
	params.Set("scope", strings.Join(a.provider.Scopes, " "))
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")

	redirectURL := a.provider.AuthURL + "?" + params.Encode()
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleCallback processes the OAuth authorization code callback.
// It retrieves the PKCE verifier from the state cache, exchanges the code for a token,
// and returns the access token to the client as JSON.
func (a *Authenticator) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		http.Error(w, "missing code parameter", http.StatusBadRequest)
		return
	}
	if state == "" {
		http.Error(w, "missing state parameter", http.StatusBadRequest)
		return
	}

	// CSRF validation: verify the state parameter matches the cookie set during
	// HandleAuthStart. This ensures only the browser that initiated the flow can
	// complete it — an attacker cannot forge the HttpOnly cookie from a different
	// origin.
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != state {
		http.Error(w, "CSRF validation failed", http.StatusForbidden)
		return
	}
	// Clear the cookie now that it has been consumed (one-time use).
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	// Retrieve and consume PKCE entry (one-time use)
	entryVal, ok := a.stateCache.LoadAndDelete(state)
	if !ok {
		http.Error(w, "unknown or expired state", http.StatusBadRequest)
		return
	}
	entry := entryVal.(pkceEntry)
	if time.Since(entry.createdAt) > 10*time.Minute {
		http.Error(w, "state expired", http.StatusBadRequest)
		return
	}

	// Exchange code + PKCE verifier for access token
	accessToken, err := a.exchangeCode(code, entry.verifier)
	if err != nil {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"access_token": accessToken,
		"token_type":   "bearer",
	})
}

// HandleLogout invalidates the session associated with the given Bearer token.
// This provides a way to revoke access and log out users before their session
// naturally expires. The token is fetched from the userinfo endpoint to identify
// the client, and all sessions for that client are deleted.
func (a *Authenticator) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token, err := extractBearerToken(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Check token cache first for the client identity
	var clientID string
	if cached, ok := a.tokenCache.Load(token); ok {
		entry := cached.(tokenCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			clientID = entry.identity.ClientID
		}
	}

	// If not in cache, fetch identity from provider (token must be valid)
	if clientID == "" {
		identity, err := a.fetchUserIdentity(token)
		if err != nil {
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}
		clientID = identity.ClientID
	}

	// Delete all sessions for this client
	sessions, err := a.sessionStore.GetByClientID(clientID)
	if err == nil {
		for _, session := range sessions {
			a.sessionStore.Delete(session.ID)
		}
	}

	// Remove from token cache
	a.tokenCache.Delete(token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status": "logged out",
	})
}

// exchangeCode exchanges an authorization code and PKCE verifier for an access token.
// It includes the client_secret in the request body when configured (HARD-08).
// Error messages are sanitized — raw provider response bodies are never returned (HARD-05).
func (a *Authenticator) exchangeCode(code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", a.cfg.ClientID)
	if a.cfg.ClientSecret != "" {
		form.Set("client_secret", a.cfg.ClientSecret)
	}
	if a.cfg.RedirectURL != "" {
		form.Set("redirect_uri", a.cfg.RedirectURL)
	}

	req, err := http.NewRequest("POST", a.provider.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("token exchange failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("authentication error occurred")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// Sanitized: do NOT include raw provider response body or status code
		// to prevent information leakage about upstream/internal state
		return "", fmt.Errorf("authentication error occurred")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}

	token, ok := result["access_token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("token response missing access_token")
	}
	return token, nil
}

// fetchUserIdentity calls the provider's userinfo endpoint with the given Bearer token
// and returns the user's identity.
func (a *Authenticator) fetchUserIdentity(token string) (*proxy.ClientIdentity, error) {
	req, err := http.NewRequest("GET", a.provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("authentication error occurred")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user identity verification failed")
	}

	// Parse provider response — GitHub uses {id, login, email}, OIDC uses {sub, email}
	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parsing userinfo response: %w", err)
	}

	identity := &proxy.ClientIdentity{
		Role: "readonly", // default role; overridden by role resolution
	}

	// Store raw claims for claim-based role mapping
	identity.Claims = make(map[string]interface{})
	for k, v := range info {
		identity.Claims[k] = v
	}

	// ClientID: prefer "id" (GitHub integer → string), fallback "sub" (OIDC), then "login"
	switch v := info["id"].(type) {
	case string:
		identity.ClientID = v
	case float64:
		identity.ClientID = fmt.Sprintf("%.0f", v)
	default:
		if sub, ok := info["sub"].(string); ok {
			identity.ClientID = sub
		} else if login, ok := info["login"].(string); ok {
			identity.ClientID = login
		}
	}

	if email, ok := info["email"].(string); ok {
		identity.Email = email
	}

	if identity.ClientID == "" {
		return nil, fmt.Errorf("userinfo response missing user identifier")
	}

	return identity, nil
}

// extractBearerToken extracts the Bearer token from the Authorization header.
// Returns an error if the header is absent, empty, or not a Bearer scheme.
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("Authorization header is required")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", fmt.Errorf("Authorization header must use Bearer scheme")
	}

	token := strings.TrimPrefix(authHeader, prefix)
	if token == "" {
		return "", fmt.Errorf("Bearer token is empty")
	}
	return token, nil
}
