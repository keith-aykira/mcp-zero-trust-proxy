// Package auth implements OAuth 2.1 PKCE authentication and per-client session isolation.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// GenerateCodeVerifier generates a cryptographically random PKCE code verifier.
// The verifier is 32 random bytes encoded as base64url (no padding), producing
// a 43-character URL-safe string compliant with RFC 7636 §4.1.
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge derives the S256 PKCE code challenge from a verifier.
// It computes SHA-256(verifier) and base64url-encodes the result (no padding),
// per RFC 7636 §4.2.
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// VerifyCodeChallenge checks that the given verifier matches the stored challenge.
// Uses constant-time comparison to prevent timing attacks.
// Returns false if either argument is empty.
func VerifyCodeChallenge(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	expected := GenerateCodeChallenge(verifier)
	// subtle.ConstantTimeCompare requires equal-length slices; compare as bytes
	return subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) == 1
}
