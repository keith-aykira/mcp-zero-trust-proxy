package ratelimit_test

import (
	"sync"
	"testing"

	"golang.org/x/time/rate"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/ratelimit"
)

// makeLimiter creates a rate limiter from config.
func makeLimiter(rpmRate int, burst int) *ratelimit.Limiter {
	cfg := &config.RateLimitConfig{
		RequestsPerMinute: rpmRate,
		BurstSize:         burst,
	}
	return ratelimit.NewLimiter(cfg)
}

// TestAllow_UnderLimit verifies a client under the rate limit is allowed.
func TestAllow_UnderLimit(t *testing.T) {
	l := makeLimiter(600, 10) // 10 req/sec, burst 10
	for i := 0; i < 5; i++ {
		if !l.Allow("client-a") {
			t.Errorf("request %d should be allowed (under limit), but was denied", i+1)
		}
	}
}

// TestAllow_ExceedsLimit verifies that exceeding burst causes denial.
func TestAllow_ExceedsLimit(t *testing.T) {
	// Use a very low rate (6 req/min = 0.1 req/sec) with burst of 2.
	// After 2 requests, the burst is exhausted and further requests are denied.
	l := makeLimiter(6, 2)

	// First 2 should be allowed (burst).
	allowed := 0
	denied := 0
	for i := 0; i < 10; i++ {
		if l.Allow("client-b") {
			allowed++
		} else {
			denied++
		}
	}
	if allowed > 3 {
		t.Errorf("expected at most 3 allowed (burst=2, ~0.1 req/sec), got %d allowed", allowed)
	}
	if denied == 0 {
		t.Error("expected some requests to be denied, but all were allowed")
	}
}

// TestAllow_PerClient verifies that limits are isolated per client.
func TestAllow_PerClient(t *testing.T) {
	// Very tight rate: 6 req/min = 0.1 req/sec, burst 1.
	l := makeLimiter(6, 1)

	// Exhaust client-A's burst.
	l.Allow("client-a")
	l.Allow("client-a")
	l.Allow("client-a")

	// Client-B should still have its full burst available.
	if !l.Allow("client-b") {
		t.Error("client-b should have its own fresh limiter and be allowed")
	}
}

// TestAllow_NewClientAllowed verifies a new/unknown client gets a fresh limiter.
func TestAllow_NewClientAllowed(t *testing.T) {
	l := makeLimiter(600, 10) // high rate
	// Brand new client never seen before.
	if !l.Allow("brand-new-client-xyz") {
		t.Error("new client should be allowed on first request")
	}
}

// TestAllow_BurstAllowsSpikes verifies burst allows short spikes.
func TestAllow_BurstAllowsSpikes(t *testing.T) {
	// 1 req/min (very slow sustained), but burst of 5.
	// All 5 burst requests should succeed immediately.
	l := makeLimiter(1, 5)

	successes := 0
	for i := 0; i < 5; i++ {
		if l.Allow("burst-client") {
			successes++
		}
	}
	if successes < 5 {
		t.Errorf("burst of 5 should allow 5 requests, got %d", successes)
	}
}

// TestAllow_ConfigurableRates verifies that rates from config are applied.
// Uses the golang.org/x/time/rate package directly to compute expected behavior.
func TestAllow_ConfigurableRates(t *testing.T) {
	// 60 req/min = 1 req/sec, burst 5.
	limiterCfg := &config.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 5}
	l := ratelimit.NewLimiter(limiterCfg)

	// Expected: 60/60 = 1.0 req/sec token rate.
	// Independently verify via rate.NewLimiter.
	expected := rate.NewLimiter(rate.Limit(60.0/60.0), 5)
	for i := 0; i < 5; i++ {
		got := l.Allow("config-client")
		want := expected.Allow()
		if got != want {
			t.Errorf("request %d: Allow() = %v, expected %v", i+1, got, want)
		}
	}
}

// TestAllow_ConcurrentSafe verifies concurrent calls don't panic or race.
func TestAllow_ConcurrentSafe(t *testing.T) {
	l := makeLimiter(6000, 100) // high enough to allow all
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientID := "concurrent-client"
			for j := 0; j < 10; j++ {
				l.Allow(clientID)
			}
		}(i)
	}
	wg.Wait()
}
