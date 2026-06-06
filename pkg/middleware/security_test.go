package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// TestRateLimiter_RedisOFF tests Scenario 1: Redis is unavailable at boot (rdb == nil).
// It verifies that fallback limiter activates, doesn't panic, and functions perfectly.
func TestRateLimiter_RedisOFF(t *testing.T) {
	// Purge global fallback cache and reset circuit breaker to ensure 100% isolated tests
	fallbackCache.Purge()
	redisBreaker.Reset()

	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RateLimiter(nil, 20, 5))

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Loop and send requests until we receive a 429 Too Many Requests.
	// Since FallbackBurst is 10, we expect throttling to occur very quickly.
	throttled := false
	for i := 1; i <= 30; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			throttled = true
			break
		}
	}

	if !throttled {
		t.Fatalf("Expected fallback rate limiter to throttle requests within 30 attempts, but all passed.")
	}
}

// TestRateLimiter_RedisOffline tests Scenario 3: Redis is initialized but fails at runtime.
// It verifies that the Circuit Breaker intercepts connection errors and degrades cleanly to the fallback cache without panics.
func TestRateLimiter_RedisOffline(t *testing.T) {
	// Purge global fallback cache and reset circuit breaker to ensure 100% isolated tests
	fallbackCache.Purge()
	redisBreaker.Reset()

	gin.SetMode(gin.TestMode)

	// Create a non-nil client pointing to a completely dead/offline local port
	badRdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:9999",
	})

	r := gin.New()
	r.Use(RateLimiter(badRdb, 20, 5))

	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Loop and send requests until we receive a 429 Too Many Requests.
	// We allow up to 30 requests to naturally absorb any token bucket refills during connection failures.
	throttled := false
	for i := 1; i <= 30; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			throttled = true
			break
		}
	}

	if !throttled {
		t.Fatalf("Expected rate limiter to throttle requests within 30 attempts under offline degradation, but all passed.")
	}
}
