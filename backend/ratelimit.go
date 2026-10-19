package main

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter is a per-key token bucket. Keys combine a bucket name (route
// class) with the client IP, so each endpoint gets its own per-IP budget.
type rateLimiter struct {
	buckets map[string]*tokenBucket
	mu      sync.Mutex
	rate    float64 // tokens added per second
	burst   float64 // maximum tokens
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

func newRateLimiter(rate float64, burst float64) *rateLimiter {
	limiter := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}

	// Prune idle buckets so the map does not grow without bound.
	go func() {
		for range time.Tick(time.Minute) {
			limiter.mu.Lock()
			for key, bucket := range limiter.buckets {
				if time.Since(bucket.lastFill) > 10*time.Minute {
					delete(limiter.buckets, key)
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

func (limiter *rateLimiter) allow(key string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()

	bucket, ok := limiter.buckets[key]
	if !ok {
		bucket = &tokenBucket{tokens: limiter.burst, lastFill: now}
		limiter.buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.lastFill).Seconds()
	bucket.tokens = min(bucket.tokens+elapsed*limiter.rate, limiter.burst)
	bucket.lastFill = now

	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--

	return true
}

// rateLimit wraps a handler with per-IP token-bucket rate limiting. name is
// the route class; rate is tokens per second and burst the bucket capacity.
func rateLimit(name string, rate float64, burst float64, next http.HandlerFunc) http.HandlerFunc {
	limiter := newRateLimiter(rate, burst)

	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		if !limiter.allow(name+"|"+ip) {
			w.Header().Set("Retry-After", "2")
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
