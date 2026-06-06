package middleware

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
	"nutrix-backend/pkg/resilience"
)

// SecurityHeaders sets basic HTTP security headers.
func SecurityHeaders(apiOnly bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		if !apiOnly {
			c.Writer.Header().Set("Content-Security-Policy", "default-src 'self';")
		}
		c.Next()
	}
}

// RequireHTTPS enforces HTTPS in production environments.
func RequireHTTPS(isProd bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isProd {
			c.Next()
			return
		}
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUpgradeRequired, gin.H{"error": "HTTPS is required"})
	}
}

// MaxBodySize limits the size of the request body.
func MaxBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

// Fallback Settings: Stricter limits during outages (50 req/min)
const (
	FallbackRPS   = 50
	FallbackBurst = 10
	LRUMaxSize    = 5000
	LRUTTL        = 60 * time.Second
)

var (
	// Fallback Cache: Protected from OOM and IP spoofing
	fallbackCache = expirable.NewLRU[string, *rate.Limiter](LRUMaxSize, nil, LRUTTL)
	
	// Global Breaker for Redis Rate Limiter
	redisBreaker = resilience.NewCircuitBreaker(5, 5*time.Second, 10*time.Second, 2)

	// ErrThrottled is returned when the request rate limit is exceeded
	ErrThrottled = errors.New("throttled")

	// Lock-free atomic timestamp to throttle fallback log warnings (at most once per 30s)
	lastWarningTime int64
)

// RateLimiter implements a Fintech-grade, resilient rate limiting tier.
// It uses Redis as primary store, but falls back to a memory-safe LRU cache if Redis is down.
func RateLimiter(rdb *redis.Client, rps int, burst int) gin.HandlerFunc {
	var limiter *redis_rate.Limiter
	if rdb != nil {
		limiter = redis_rate.NewLimiter(rdb)
	}

	return func(c *gin.Context) {
		key := getRateLimitKey(c)

		if limiter == nil {
			// Redis was unavailable at boot, fallback to in-memory LRU immediately
			handleFallback(c, key)
			return
		}

		limit := redis_rate.Limit{
			Rate:   rps,
			Burst:  burst,
			Period: time.Minute,
		}

		// Wrap Redis call in a Circuit Breaker with a strict 500ms timeout
		err := redisBreaker.Execute(c.Request.Context(), func() error {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
			defer cancel()

			res, err := limiter.Allow(ctx, key, limit)
			if err != nil {
				return err
			}

			if res.Allowed <= 0 {
				logRateLimitExceeded(key, c.ClientIP())
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded. Too many requests.",
				})
				return ErrThrottled
			}
			return nil
		})

		if err != nil {
			if errors.Is(err, ErrThrottled) {
				return
			}

			// Handle Redis Failure / Circuit Open: Graceful Degradation to LRU Fallback
			handleFallback(c, key)
			return
		}

		c.Next()
	}
}

func getRateLimitKey(c *gin.Context) string {
	// 1. Authenticated User Key
	if userID, exists := c.Get("userID"); exists {
		return fmt.Sprintf("rate:user:%v", userID)
	}

	// 2. Public Route Key: Anti-NAT Fingerprinting (IP + User-Agent Hash)
	ua := c.Request.UserAgent()
	uaHash := fmt.Sprintf("%x", sha256.Sum256([]byte(ua)))
	return fmt.Sprintf("rate:ip:%s:%s", c.ClientIP(), uaHash[:8])
}

func handleFallback(c *gin.Context, key string) {
	// Circuit Breaker Logging is handled during state transitions.
	// Suppress logging warning to at most once per 30 seconds to prevent console spam.
	now := time.Now().Unix()
	last := atomic.LoadInt64(&lastWarningTime)
	if now-last >= 30 {
		if atomic.CompareAndSwapInt64(&lastWarningTime, last, now) {
			log.Printf("[SRE WARNING] Redis Limiter is failing. Degrading to LRU fallback (suppressing warning spam for 30s).")
		}
	}

	// Stricter In-Memory Fallback
	val, ok := fallbackCache.Get(key)
	if !ok {
		val = rate.NewLimiter(rate.Every(time.Minute/FallbackRPS), FallbackBurst)
		fallbackCache.Add(key, val)
	}

	if !val.Allow() {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": "Rate limit exceeded (Fallback).",
		})
		return
	}
	c.Next()
}

func logRateLimitExceeded(key, ip string) {
	// Structured logging for monitoring ingestion
	log.Printf(`{"event":"rate_limit_exceeded", "key":"%s", "ip":"%s"}`, key, ip)
}
