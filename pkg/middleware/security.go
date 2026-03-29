package middleware

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// MaxBodySize wraps the request body using http.MaxBytesReader
// Anything larger than the configured limit bytes will trigger an error when parsing.
func MaxBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

// client tracks rate limiting specific to an IP address,
// along with the last time we saw a request from this IP.
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter implements a robust token bucket algorithm using golang.org/x/time/rate.
// It tracks limiters per IP address using a thread-safe map, and periodically sweeps
// the map in a background goroutine to clean up inactive IPs and prevent Memory Leaks.
func RateLimiter() gin.HandlerFunc {
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Spawns a background Goroutine that runs repeatedly to prevent OOM
	go func() {
		for {
			time.Sleep(5 * time.Minute)

			mu.Lock()
			for ip, c := range clients {
				// If an IP hasn't been active in 10 minutes, delete its limiter
				if time.Since(c.lastSeen) > 10*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
			log.Println("[Security] Background cleanup sweeping inactive RateLimiter IPs...")
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		if _, found := clients[ip]; !found {
			// e.g., Set to 100 requests per minute with a burst of 100
			clients[ip] = &client{
				limiter: rate.NewLimiter(rate.Every(time.Minute/100), 100),
			}
		}

		clients[ip].lastSeen = time.Now()
		limiter := clients[ip].limiter
		mu.Unlock() // unlock quickly so we don't hold it during processing

		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Too many requests.",
			})
			return
		}

		c.Next()
	}
}
