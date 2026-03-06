package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/atlaspay/platform/internal/common/errors"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	buckets    map[string]*bucket
	mu         sync.RWMutex
	rate       int           // tokens per interval
	interval   time.Duration // refill interval
	bucketSize int           // max tokens
}

type bucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
// rate: requests allowed per interval
// interval: time window
// bucketSize: max burst size
func NewRateLimiter(rate int, interval time.Duration, bucketSize int) *RateLimiter {
	rl := &RateLimiter{
		buckets:    make(map[string]*bucket),
		rate:       rate,
		interval:   interval,
		bucketSize: bucketSize,
	}

	// Cleanup old buckets periodically
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		for key, b := range rl.buckets {
			if time.Since(b.lastRefill) > 10*time.Minute {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) getBucket(key string) *bucket {
	rl.mu.RLock()
	b, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if exists {
		return b
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double check after acquiring write lock
	if b, exists = rl.buckets[key]; exists {
		return b
	}

	b = &bucket{
		tokens:     rl.bucketSize,
		lastRefill: time.Now(),
	}
	rl.buckets[key] = b
	return b
}

// Allow checks if the request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	b := rl.getBucket(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	tokensToAdd := int(elapsed/rl.interval) * rl.rate

	if tokensToAdd > 0 {
		b.tokens = min(rl.bucketSize, b.tokens+tokensToAdd)
		b.lastRefill = now
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// RateLimit creates rate limiting middleware
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use IP as key, or user ID if authenticated
			key := r.RemoteAddr
			if r.Header.Get("X-Forwarded-For") != "" {
				key = r.Header.Get("X-Forwarded-For")
			}

			if !limiter.Allow(key) {
				w.Header().Set("Retry-After", "60")
				errors.WriteError(w, &errors.AppError{
					Code:    http.StatusTooManyRequests,
					Message: "Rate limit exceeded",
					Details: "Please try again later",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
