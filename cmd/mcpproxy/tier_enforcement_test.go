package main

import (
	"testing"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/license"
)

// applyTierOverrides applies tier-based config overrides, matching the logic in main().
// This is extracted here for testability only — it must stay in sync with main.go.
func applyTierOverrides(lic *license.License, cfg *config.Config) {
	switch lic.Tier {
	case license.TierFree:
		// Free tier: cap rate limit to 60 req/min, force audit to stdout only.
		// 60 req/min allows AI agent workflows (Claude, Cursor) to complete without hitting limits.
		cfg.RateLimit.RequestsPerMinute = 60
		if cfg.RateLimit.BurstSize > 30 {
			cfg.RateLimit.BurstSize = 30
		}
		cfg.Audit.Output = "stdout"
		cfg.Audit.FilePath = ""
	case license.TierPro:
		if lic.MaxRPM > 0 && cfg.RateLimit.RequestsPerMinute > lic.MaxRPM {
			cfg.RateLimit.RequestsPerMinute = lic.MaxRPM
		}
	case license.TierEnterprise:
		// Enterprise tier: no caps — all configured values are honoured.
	}
}

// TestFreeTierRateLimit verifies that the free tier enforces 60 req/min and burst 30.
func TestFreeTierRateLimit(t *testing.T) {
	lic := &license.License{Tier: license.TierFree}
	cfg := &config.Config{}
	cfg.RateLimit.RequestsPerMinute = 100 // will be overridden
	cfg.RateLimit.BurstSize = 50          // will be capped to 30

	applyTierOverrides(lic, cfg)

	if cfg.RateLimit.RequestsPerMinute != 60 {
		t.Errorf("free tier: expected RequestsPerMinute=60, got %d", cfg.RateLimit.RequestsPerMinute)
	}
	if cfg.RateLimit.BurstSize != 30 {
		t.Errorf("free tier: expected BurstSize=30, got %d", cfg.RateLimit.BurstSize)
	}
	if cfg.Audit.Output != "stdout" {
		t.Errorf("free tier: expected Audit.Output=stdout, got %q", cfg.Audit.Output)
	}
}

// TestFreeTierBurstUnderLimit verifies that burst size is NOT capped when already <= 30.
func TestFreeTierBurstUnderLimit(t *testing.T) {
	lic := &license.License{Tier: license.TierFree}
	cfg := &config.Config{}
	cfg.RateLimit.RequestsPerMinute = 100
	cfg.RateLimit.BurstSize = 10 // already under 30 — should stay 10

	applyTierOverrides(lic, cfg)

	if cfg.RateLimit.BurstSize != 10 {
		t.Errorf("free tier burst under limit: expected BurstSize=10, got %d", cfg.RateLimit.BurstSize)
	}
}

// TestProTierNotAffected verifies the Pro tier does not apply free-tier overrides.
func TestProTierNotAffected(t *testing.T) {
	lic := &license.License{Tier: license.TierPro, MaxRPM: 200}
	cfg := &config.Config{}
	cfg.RateLimit.RequestsPerMinute = 100 // within pro limits, not changed
	cfg.RateLimit.BurstSize = 50

	applyTierOverrides(lic, cfg)

	// Pro tier: RPM should not be overridden (100 < 200 MaxRPM)
	if cfg.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("pro tier: expected RequestsPerMinute=100 (unchanged), got %d", cfg.RateLimit.RequestsPerMinute)
	}
}
