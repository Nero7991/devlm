package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	ips    map[string]*rateLimiterEntry
	mu     sync.RWMutex
	rate   rate.Limit
	burst  int
	expiry time.Duration
}

type rateLimiterEntry struct {
	limiter *rate.Limiter
	lastHit time.Time
}

func NewRateLimiter(r rate.Limit, b int, expiry time.Duration) *RateLimiter {
	rl := &RateLimiter{
		ips:    make(map[string]*rateLimiterEntry),
		rate:   r,
		burst:  b,
		expiry: expiry,
	}
	go rl.periodicCleanup()
	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	entry, exists := rl.ips[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		defer rl.mu.Unlock()
		entry, exists = rl.ips[ip]
		if !exists {
			limiter := rate.NewLimiter(rl.rate, rl.burst)
			entry = &rateLimiterEntry{
				limiter: limiter,
				lastHit: time.Now(),
			}
			rl.ips[ip] = entry
		}
	}

	entry.lastHit = time.Now()
	return entry.limiter
}

func (rl *RateLimiter) periodicCleanup() {
	ticker := time.NewTicker(rl.expiry / 2)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, entry := range rl.ips {
			if time.Since(entry.lastHit) > rl.expiry {
				delete(rl.ips, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) RateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("X-RateLimit-Limit", limiter.Limit().String())
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", time.Now().Add(time.Second*60).Format(time.RFC1123))

			response := map[string]string{
				"error":   "Rate limit exceeded",
				"message": "Please try again later",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(response)
			return
		}

		w.Header().Set("X-RateLimit-Limit", limiter.Limit().String())
		w.Header().Set("X-RateLimit-Remaining", "1")
		next.ServeHTTP(w, r)
	}
}

func (rl *RateLimiter) GetLimitForIP(ip string) (rate.Limit, int, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if entry, exists := rl.ips[ip]; exists {
		return entry.limiter.Limit(), entry.limiter.Burst(), true
	}
	return 0, 0, false
}

func (rl *RateLimiter) SetLimitForIP(ip string, r rate.Limit, b int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if r <= 0 || b < 0 {
		return
	}

	if entry, exists := rl.ips[ip]; exists {
		entry.limiter.SetLimit(r)
		entry.limiter.SetBurst(b)
	} else {
		limiter := rate.NewLimiter(r, b)
		rl.ips[ip] = &rateLimiterEntry{
			limiter: limiter,
			lastHit: time.Now(),
		}
	}
}

func (rl *RateLimiter) RemoveLimitForIP(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if _, exists := rl.ips[ip]; exists {
		delete(rl.ips, ip)
		return true
	}
	return false
}

func (rl *RateLimiter) GetAllActiveLimits() map[string]rate.Limit {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	limits := make(map[string]rate.Limit)
	for ip, entry := range rl.ips {
		limits[ip] = entry.limiter.Limit()
	}
	return limits
}

func (rl *RateLimiter) GetRemainingQuota(ip string) (int, bool) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if entry, exists := rl.ips[ip]; exists {
		return entry.limiter.Burst() - entry.limiter.Tokens(), true
	}
	return 0, false
}

func (rl *RateLimiter) ResetLimitForIP(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if entry, exists := rl.ips[ip]; exists {
		entry.limiter = rate.NewLimiter(rl.rate, rl.burst)
		entry.lastHit = time.Now()
		return true
	}
	return false
}