package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type IPRateLimiter struct {
	visits          map[string][]time.Time
	mu              sync.Mutex
	limit           int
	window          time.Duration
	cleanupInterval time.Duration
	stopChan        chan struct{}
	stopOnce        sync.Once
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	rl := &IPRateLimiter{
		visits:          make(map[string][]time.Time),
		limit:           limit,
		window:          window,
		cleanupInterval: 5 * time.Minute,
		stopChan:        make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (i *IPRateLimiter) Allow(ip string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-i.window)
	visits := i.visits[ip]
	valid := visits[:0]
	for _, t := range visits {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= i.limit {
		i.visits[ip] = valid
		return false
	}
	i.visits[ip] = append(valid, now)
	return true
}

func (i *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(i.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			i.pruneOldEntries()
		case <-i.stopChan:
			return
		}
	}
}

func (i *IPRateLimiter) pruneOldEntries() {
	i.mu.Lock()
	defer i.mu.Unlock()
	cutoff := time.Now().Add(-i.window)
	for ip, visits := range i.visits {
		if len(visits) == 0 || visits[len(visits)-1].Before(cutoff) {
			delete(i.visits, ip)
		}
	}
}

func (i *IPRateLimiter) Stop() { i.stopOnce.Do(func() { close(i.stopChan) }) }

func RateLimitMiddleware(rl *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Allow(clientIP(c)) {
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "message": "Terlalu banyak percobaan. Silakan coba lagi nanti."})
			c.Abort()
			return
		}
		c.Next()
	}
}

var LoginRateLimit = NewIPRateLimiter(5, time.Minute)

func RateLimitLogin() gin.HandlerFunc { return RateLimitMiddleware(LoginRateLimit) }

// clientIP deliberately delegates proxy handling to Gin. Configure trusted proxies
// in main.go; never trust X-Forwarded-For directly from an arbitrary client.
func clientIP(c *gin.Context) string {
	return strings.TrimSpace(c.ClientIP())
}
