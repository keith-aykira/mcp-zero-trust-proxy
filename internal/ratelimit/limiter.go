package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
)

// clientState holds the rate.Limiter for a single client and a last-accessed time
// for future cleanup.
type clientState struct {
	limiter        *rate.Limiter
	lastAccessUnix atomic.Int64 // Unix nanoseconds, updated atomically
}

// Limiter enforces per-client request rate limits using the token bucket algorithm.
// It implements the middleware.RateLimiter interface via Allow().
// All operations are safe for concurrent use.
type Limiter struct {
	defaultRate  rate.Limit
	defaultBurst int
	clients      sync.Map // string -> *clientState
}

// NewLimiter creates a Limiter from a RateLimitConfig.
// Converts requests_per_minute to a per-second rate for the token bucket.
func NewLimiter(cfg *config.RateLimitConfig) *Limiter {
	rpm := cfg.RequestsPerMinute
	if rpm <= 0 {
		rpm = 300 // default: 300 req/min
	}
	burst := cfg.BurstSize
	if burst <= 0 {
		burst = 100 // default: burst of 100
	}

	return &Limiter{
		defaultRate:  rate.Limit(float64(rpm) / 60.0),
		defaultBurst: burst,
	}
}

// Allow returns true if the client is within their rate limit.
// Returns false if the request should be throttled.
// If the client is not yet known, a fresh limiter is created for them.
func (l *Limiter) Allow(clientID string) bool {
	return l.getOrCreate(clientID).Allow()
}

// getOrCreate atomically retrieves or creates the rate.Limiter for a client.
// Uses sync.Map.LoadOrStore for thread-safe lazy initialization.
func (l *Limiter) getOrCreate(clientID string) *rate.Limiter {
	// Fast path: already exists.
	if v, ok := l.clients.Load(clientID); ok {
		cs := v.(*clientState)
		// Update last-access time atomically (no mutex needed for int64).
		cs.lastAccessUnix.Store(time.Now().UnixNano())
		return cs.limiter
	}

	// Slow path: create a new limiter and store atomically.
	newCS := &clientState{
		limiter: rate.NewLimiter(l.defaultRate, l.defaultBurst),
	}
	newCS.lastAccessUnix.Store(time.Now().UnixNano())
	// LoadOrStore: if another goroutine already stored one between our Load and Store,
	// we use theirs (and our newCS is discarded).
	actual, _ := l.clients.LoadOrStore(clientID, newCS)
	return actual.(*clientState).limiter
}

// Cleanup removes client limiters that have not been accessed within the staleness window.
// Call periodically from a background goroutine or on-demand to prevent unbounded growth.
func (l *Limiter) Cleanup(staleAfter time.Duration) {
	cutoffNano := time.Now().Add(-staleAfter).UnixNano()
	l.clients.Range(func(key, value interface{}) bool {
		cs := value.(*clientState)
		if cs.lastAccessUnix.Load() < cutoffNano {
			l.clients.Delete(key)
		}
		return true
	})
}
