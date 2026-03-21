package license

import (
	"crypto/ecdsa"
	"time"
)

// Tier represents a license tier.
type Tier string

const (
	// TierFree is the default tier with no license key — 1 upstream, 10 req/min.
	TierFree Tier = "free"
	// TierPro is the paid tier — 5 upstreams, up to 200 req/min, audit file output.
	TierPro Tier = "pro"
	// TierEnterprise is the unlimited tier — no upstreams limit, no rate cap, all features.
	TierEnterprise Tier = "enterprise"
)

// License represents the decoded, validated claims from a license JWT.
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
func FreeTierLicense() *License {
	panic("not implemented")
}

// Parse decodes and validates a license key string.
// An empty keyString returns a free-tier License with no error.
// A non-empty keyString must be a valid ECDSA P-256 signed JWT.
func Parse(keyString string, publicKey *ecdsa.PublicKey) (*License, error) {
	panic("not implemented")
}

// Validate checks that the license has not expired.
// It returns an error if the license's ExpiresAt is in the past.
// A zero ExpiresAt (free tier) is always valid.
func Validate(lic *License) error {
	panic("not implemented")
}

// SignJWT signs the sigInput string (header.payload) with the private key
// and returns the base64url-encoded DER signature.
// This is exported for use in tests only.
func SignJWT(priv *ecdsa.PrivateKey, sigInput string) (string, error) {
	panic("not implemented")
}
