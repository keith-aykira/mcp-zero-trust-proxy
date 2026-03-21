package license_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/license"
)

// generateTestKey produces a fresh ECDSA P-256 key pair for testing.
func generateTestKey(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return priv, &priv.PublicKey
}

// makeJWT builds a minimal JWT with the given claims, signed by priv.
// header: {"alg":"ES256","typ":"JWT"}
func makeJWT(t *testing.T, priv *ecdsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()

	headerJSON, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": "JWT"})
	claimsJSON, _ := json.Marshal(claims)

	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	sigInput := header + "." + payload

	sig, err := license.SignJWT(priv, sigInput)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	return sigInput + "." + sig
}

// --- Tests ---

// TestFreeTier_EmptyKey verifies that Parse("", ...) returns a free-tier License with no error.
func TestFreeTier_EmptyKey(t *testing.T) {
	_, pub := generateTestKey(t)

	lic, err := license.Parse("", pub)
	if err != nil {
		t.Fatalf("Parse(\"\") should return free tier, got error: %v", err)
	}
	if lic.Tier != license.TierFree {
		t.Errorf("expected tier %q, got %q", license.TierFree, lic.Tier)
	}
	if lic.MaxUpstreams != 1 {
		t.Errorf("free tier: expected MaxUpstreams=1, got %d", lic.MaxUpstreams)
	}
	if lic.MaxRPM != 10 {
		t.Errorf("free tier: expected MaxRPM=10, got %d", lic.MaxRPM)
	}
}

// TestProTier_ValidJWT verifies that a valid Pro-tier JWT returns the correct license.
func TestProTier_ValidJWT(t *testing.T) {
	priv, pub := generateTestKey(t)

	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	claims := map[string]interface{}{
		"tier":          "pro",
		"max_upstreams": 5,
		"max_rpm":       200,
		"exp":           exp,
		"iss":           "mcpzerotrust.dev",
		"sub":           "customer@example.com",
	}
	token := makeJWT(t, priv, claims)

	lic, err := license.Parse(token, pub)
	if err != nil {
		t.Fatalf("Parse(validProJWT) should succeed, got error: %v", err)
	}
	if lic.Tier != license.TierPro {
		t.Errorf("expected tier %q, got %q", license.TierPro, lic.Tier)
	}
	if lic.MaxUpstreams != 5 {
		t.Errorf("expected MaxUpstreams=5, got %d", lic.MaxUpstreams)
	}
	if lic.MaxRPM != 200 {
		t.Errorf("expected MaxRPM=200, got %d", lic.MaxRPM)
	}
	if lic.Subject != "customer@example.com" {
		t.Errorf("expected Subject=%q, got %q", "customer@example.com", lic.Subject)
	}
}

// TestEnterpriseTier_ValidJWT verifies that a valid Enterprise-tier JWT returns the correct license.
func TestEnterpriseTier_ValidJWT(t *testing.T) {
	priv, pub := generateTestKey(t)

	exp := time.Now().Add(365 * 24 * time.Hour).Unix()
	claims := map[string]interface{}{
		"tier":          "enterprise",
		"max_upstreams": 0, // 0 = unlimited
		"max_rpm":       0, // 0 = unlimited
		"exp":           exp,
		"iss":           "mcpzerotrust.dev",
		"sub":           "enterprise@example.com",
	}
	token := makeJWT(t, priv, claims)

	lic, err := license.Parse(token, pub)
	if err != nil {
		t.Fatalf("Parse(validEnterpriseJWT) should succeed, got error: %v", err)
	}
	if lic.Tier != license.TierEnterprise {
		t.Errorf("expected tier %q, got %q", license.TierEnterprise, lic.Tier)
	}
	if lic.MaxUpstreams != 0 {
		t.Errorf("enterprise tier: expected MaxUpstreams=0 (unlimited), got %d", lic.MaxUpstreams)
	}
	if lic.MaxRPM != 0 {
		t.Errorf("enterprise tier: expected MaxRPM=0 (unlimited), got %d", lic.MaxRPM)
	}
}

// TestExpiredJWT verifies that an expired JWT returns "license expired" error.
func TestExpiredJWT(t *testing.T) {
	priv, pub := generateTestKey(t)

	// exp in the past
	exp := time.Now().Add(-24 * time.Hour).Unix()
	claims := map[string]interface{}{
		"tier":          "pro",
		"max_upstreams": 5,
		"max_rpm":       200,
		"exp":           exp,
		"iss":           "mcpzerotrust.dev",
		"sub":           "customer@example.com",
	}
	token := makeJWT(t, priv, claims)

	_, err := license.Parse(token, pub)
	if err == nil {
		t.Fatal("Parse(expiredJWT) should return error, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected error containing 'expired', got: %v", err)
	}
}

// TestTamperedJWT verifies that a JWT with a modified payload returns "invalid signature" error.
func TestTamperedJWT(t *testing.T) {
	priv, pub := generateTestKey(t)

	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	claims := map[string]interface{}{
		"tier":          "pro",
		"max_upstreams": 5,
		"max_rpm":       200,
		"exp":           exp,
		"iss":           "mcpzerotrust.dev",
		"sub":           "customer@example.com",
	}
	token := makeJWT(t, priv, claims)

	// Tamper: replace the payload with a modified one (escalate to enterprise)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
	tamperedClaims := map[string]interface{}{
		"tier":          "enterprise",
		"max_upstreams": 0,
		"max_rpm":       0,
		"exp":           exp,
		"iss":           "mcpzerotrust.dev",
		"sub":           "customer@example.com",
	}
	tamperedJSON, _ := json.Marshal(tamperedClaims)
	parts[1] = base64.RawURLEncoding.EncodeToString(tamperedJSON)
	tamperedToken := strings.Join(parts, ".")

	_, err := license.Parse(tamperedToken, pub)
	if err == nil {
		t.Fatal("Parse(tamperedJWT) should return error, got nil")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected error containing 'signature', got: %v", err)
	}
}

// TestWrongIssuer verifies that a JWT with a wrong issuer returns "invalid issuer" error.
func TestWrongIssuer(t *testing.T) {
	priv, pub := generateTestKey(t)

	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	claims := map[string]interface{}{
		"tier":          "pro",
		"max_upstreams": 5,
		"max_rpm":       200,
		"exp":           exp,
		"iss":           "evil.example.com", // wrong issuer
		"sub":           "customer@example.com",
	}
	token := makeJWT(t, priv, claims)

	_, err := license.Parse(token, pub)
	if err == nil {
		t.Fatal("Parse(wrongIssuerJWT) should return error, got nil")
	}
	if !strings.Contains(err.Error(), "issuer") {
		t.Errorf("expected error containing 'issuer', got: %v", err)
	}
}

// TestValidate_Expiry verifies that Validate returns an error for an already-expired license.
func TestValidate_Expiry(t *testing.T) {
	lic := &license.License{
		Tier:         license.TierPro,
		MaxUpstreams: 5,
		MaxRPM:       200,
		ExpiresAt:    time.Now().Add(-1 * time.Hour), // expired
		Subject:      "customer@example.com",
	}

	err := license.Validate(lic)
	if err == nil {
		t.Fatal("Validate(expiredLicense) should return error, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected error containing 'expired', got: %v", err)
	}
}

// TestValidate_Valid verifies that Validate returns nil for a valid (not-yet-expired) license.
func TestValidate_Valid(t *testing.T) {
	lic := &license.License{
		Tier:         license.TierPro,
		MaxUpstreams: 5,
		MaxRPM:       200,
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
		Subject:      "customer@example.com",
	}

	if err := license.Validate(lic); err != nil {
		t.Errorf("Validate(validLicense) should return nil, got: %v", err)
	}
}

// TestFreeTierLicense verifies the FreeTierLicense() constructor returns correct defaults.
func TestFreeTierLicense(t *testing.T) {
	lic := license.FreeTierLicense()
	if lic.Tier != license.TierFree {
		t.Errorf("expected tier %q, got %q", license.TierFree, lic.Tier)
	}
	if lic.MaxUpstreams != 1 {
		t.Errorf("expected MaxUpstreams=1, got %d", lic.MaxUpstreams)
	}
	if lic.MaxRPM != 10 {
		t.Errorf("expected MaxRPM=10, got %d", lic.MaxRPM)
	}
	// Free tier has no expiry (zero value)
	if !lic.ExpiresAt.IsZero() {
		t.Errorf("free tier ExpiresAt should be zero, got %v", lic.ExpiresAt)
	}
}

// TestWrongKey verifies that a JWT signed with a different key fails signature verification.
func TestWrongKey(t *testing.T) {
	priv, _ := generateTestKey(t)
	_, differentPub := generateTestKey(t) // different key pair

	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	claims := map[string]interface{}{
		"tier":          "pro",
		"max_upstreams": 5,
		"max_rpm":       200,
		"exp":           exp,
		"iss":           "mcpzerotrust.dev",
		"sub":           "customer@example.com",
	}
	token := makeJWT(t, priv, claims)

	_, err := license.Parse(token, differentPub)
	if err == nil {
		t.Fatal("Parse(wrongKeyJWT) should return error, got nil")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("expected error containing 'signature', got: %v", err)
	}
}

// TestMalformedJWT_TwoParts verifies that a JWT with only 2 parts returns an error.
func TestMalformedJWT_TwoParts(t *testing.T) {
	_, pub := generateTestKey(t)
	_, err := license.Parse("header.payload", pub)
	if err == nil {
		t.Fatal("Parse(malformedJWT) should return error, got nil")
	}
}
