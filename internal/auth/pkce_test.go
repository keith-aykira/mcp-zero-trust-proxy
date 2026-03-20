package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier() returned error: %v", err)
	}

	// RFC 7636 §4.1: verifier must be 43–128 characters
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("verifier length %d is outside RFC 7636 range [43, 128]", len(verifier))
	}

	// Must be URL-safe base64 (no +, /, or =)
	if strings.ContainsAny(verifier, "+/=") {
		t.Errorf("verifier %q contains non-URL-safe characters", verifier)
	}
}

func TestGenerateCodeVerifierIsRandom(t *testing.T) {
	v1, _ := GenerateCodeVerifier()
	v2, _ := GenerateCodeVerifier()
	if v1 == v2 {
		t.Error("two calls to GenerateCodeVerifier() returned identical values — not random")
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" // known test vector
	challenge := GenerateCodeChallenge(verifier)

	// Manually compute S256 challenge
	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])

	if challenge != expected {
		t.Errorf("challenge = %q, want %q", challenge, expected)
	}
}

func TestGenerateCodeChallengeURLSafe(t *testing.T) {
	verifier, _ := GenerateCodeVerifier()
	challenge := GenerateCodeChallenge(verifier)

	// Must be URL-safe base64 (no padding)
	if strings.ContainsAny(challenge, "+/=") {
		t.Errorf("challenge %q contains non-URL-safe characters", challenge)
	}
}

func TestVerifyCodeChallengeMatch(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier() error: %v", err)
	}
	challenge := GenerateCodeChallenge(verifier)

	if !VerifyCodeChallenge(verifier, challenge) {
		t.Error("VerifyCodeChallenge() returned false for matching pair")
	}
}

func TestVerifyCodeChallengeMismatch(t *testing.T) {
	verifier, _ := GenerateCodeVerifier()
	wrongVerifier, _ := GenerateCodeVerifier()
	challenge := GenerateCodeChallenge(wrongVerifier)

	if VerifyCodeChallenge(verifier, challenge) {
		t.Error("VerifyCodeChallenge() returned true for mismatched pair")
	}
}

func TestVerifyCodeChallengeEmptyInputs(t *testing.T) {
	if VerifyCodeChallenge("", "") {
		t.Error("VerifyCodeChallenge() should return false for empty inputs")
	}
	if VerifyCodeChallenge("verifier", "") {
		t.Error("VerifyCodeChallenge() should return false for empty challenge")
	}
}
