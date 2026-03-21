// Package license provides JWT license key parsing, validation, and tier enforcement
// for the MCP Zero-Trust Proxy. License keys are ECDSA P-256 signed JWTs that encode
// tier, resource limits, and expiry. The proxy enforces limits locally with zero
// network calls — all information is embedded in the signed token.
package license

import (
	_ "embed"

	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// embeddedPublicKeyPEM is the ECDSA P-256 public key embedded at build time.
// Replace keys/license-signing-public.pem to rotate the signing key.
//
//go:embed license-signing-public.pem
var embeddedPublicKeyPEM []byte

// Tier represents a subscription tier that controls feature limits.
type Tier string

const (
	// TierFree is the default tier with no license key.
	// Limits: 1 upstream server, 10 req/min, audit output = stdout only.
	TierFree Tier = "free"
	// TierPro is the paid tier ($49/mo).
	// Limits: 5 upstream servers, up to 200 req/min, audit file output enabled.
	TierPro Tier = "pro"
	// TierEnterprise is the unlimited tier ($199/mo).
	// Limits: no upstream cap, no rate cap, all features enabled.
	TierEnterprise Tier = "enterprise"
)

// expectedIssuer is the required JWT issuer claim for all license keys.
const expectedIssuer = "mcpzerotrust.dev"

// License holds the decoded, validated claims from a license JWT.
type License struct {
	// Tier is the subscription tier (free, pro, enterprise).
	Tier Tier
	// MaxUpstreams is the maximum number of upstream MCP servers allowed.
	// 0 means unlimited (Enterprise tier).
	MaxUpstreams int
	// MaxRPM is the maximum requests per minute allowed.
	// 0 means unlimited (Enterprise tier).
	MaxRPM int
	// ExpiresAt is when the license expires. Zero value means no expiry (free tier).
	ExpiresAt time.Time
	// Subject is the license holder's identifier (typically customer email).
	Subject string
}

// FreeTierLicense returns a License with free tier defaults.
// Free tier: 1 upstream, 10 req/min, no file audit, no expiry.
func FreeTierLicense() *License {
	return &License{
		Tier:         TierFree,
		MaxUpstreams: 1,
		MaxRPM:       10,
		ExpiresAt:    time.Time{}, // zero = no expiry
	}
}

// ParseEmbedded parses the license key using the public key embedded in the binary.
// This is the function called by main.go — it requires no external key material.
// An empty keyString returns a free-tier License with no error.
func ParseEmbedded(keyString string) (*License, error) {
	pub, err := PublicKeyFromPEM(embeddedPublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("license: failed to load embedded public key: %w", err)
	}
	return Parse(keyString, pub)
}

// PublicKeyFromPEM parses a PEM-encoded ECDSA P-256 public key.
// The PEM block must be of type "PUBLIC KEY" in PKIX/SubjectPublicKeyInfo format.
func PublicKeyFromPEM(pemData []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("license: no PEM block found in public key data")
	}
	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("license: expected PEM type 'PUBLIC KEY', got %q", block.Type)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("license: parsing public key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("license: public key is not an ECDSA key")
	}
	return ecKey, nil
}

// jwtClaims is the internal struct for decoding JWT payload claims.
type jwtClaims struct {
	Tier         string  `json:"tier"`
	MaxUpstreams int     `json:"max_upstreams"`
	MaxRPM       int     `json:"max_rpm"`
	Exp          float64 `json:"exp"` // Unix timestamp, JSON numbers decode as float64
	Iss          string  `json:"iss"`
	Sub          string  `json:"sub"`
}

// ecdsaSignature is the DER-encoded ASN.1 structure for ECDSA signatures.
type ecdsaSignature struct {
	R, S *big.Int
}

// Parse decodes and validates a license key string using the provided public key.
//
// An empty keyString returns a free-tier License with no error.
//
// A non-empty keyString must be a valid ECDSA P-256 signed JWT with:
//   - alg: ES256
//   - iss: "mcpzerotrust.dev"
//   - exp: a future Unix timestamp
//
// Errors are returned for: malformed tokens, invalid signatures,
// wrong issuer, and expired tokens.
func Parse(keyString string, publicKey *ecdsa.PublicKey) (*License, error) {
	if strings.TrimSpace(keyString) == "" {
		return FreeTierLicense(), nil
	}

	// Split into the 3 JWT parts: header.payload.signature
	parts := strings.Split(keyString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed license key: expected 3 JWT parts, got %d", len(parts))
	}

	sigInput := parts[0] + "." + parts[1]

	// Decode the signature (DER-encoded ASN.1 ECDSA signature)
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("license key: invalid signature encoding: %w", err)
	}

	// Parse the ASN.1 DER-encoded signature into R, S components
	var sig ecdsaSignature
	if _, err := asn1.Unmarshal(sigBytes, &sig); err != nil {
		return nil, fmt.Errorf("license key: invalid signature format: %w", err)
	}

	// Verify the ECDSA signature: hash(header.payload)
	digest := sha256.Sum256([]byte(sigInput))
	if !ecdsa.Verify(publicKey, digest[:], sig.R, sig.S) {
		return nil, fmt.Errorf("license key: invalid signature")
	}

	// Decode the payload (base64url -> JSON -> claims)
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("license key: invalid payload encoding: %w", err)
	}

	var claims jwtClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("license key: invalid payload JSON: %w", err)
	}

	// Validate issuer
	if claims.Iss != expectedIssuer {
		return nil, fmt.Errorf("license key: invalid issuer %q (expected %q)", claims.Iss, expectedIssuer)
	}

	// Validate expiry
	if claims.Exp == 0 {
		return nil, fmt.Errorf("license key: missing expiry claim")
	}
	expiresAt := time.Unix(int64(claims.Exp), 0)
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("license key: license expired at %s", expiresAt.UTC().Format(time.RFC3339))
	}

	// Map tier string to Tier type
	tier, err := parseTier(claims.Tier)
	if err != nil {
		return nil, err
	}

	return &License{
		Tier:         tier,
		MaxUpstreams: claims.MaxUpstreams,
		MaxRPM:       claims.MaxRPM,
		ExpiresAt:    expiresAt,
		Subject:      claims.Sub,
	}, nil
}

// Validate checks that the license has not expired.
// Returns an error if ExpiresAt is non-zero and in the past.
// A zero ExpiresAt (free tier) is always valid.
func Validate(lic *License) error {
	if lic.ExpiresAt.IsZero() {
		return nil // free tier — no expiry
	}
	if time.Now().After(lic.ExpiresAt) {
		return fmt.Errorf("license expired at %s", lic.ExpiresAt.UTC().Format(time.RFC3339))
	}
	return nil
}

// SignJWT signs the sigInput string (header.payload) with the private key
// and returns the base64url-encoded DER-encoded ASN.1 ECDSA signature.
//
// This is exported for use in tests only. In production, license keys are
// generated offline and distributed to customers.
func SignJWT(priv *ecdsa.PrivateKey, sigInput string) (string, error) {
	digest := sha256.Sum256([]byte(sigInput))

	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	// Encode as ASN.1 DER
	derSig, err := asn1.Marshal(ecdsaSignature{R: r, S: s})
	if err != nil {
		return "", fmt.Errorf("encoding signature: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(derSig), nil
}

// parseTier maps a tier string from JWT claims to the Tier type.
func parseTier(s string) (Tier, error) {
	switch Tier(s) {
	case TierFree, TierPro, TierEnterprise:
		return Tier(s), nil
	default:
		return "", fmt.Errorf("license key: unknown tier %q", s)
	}
}
