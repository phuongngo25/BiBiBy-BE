package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMiddleware collects API request metrics.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Collect metrics after request is processed
		duration := time.Since(start).Milliseconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		endpoint := c.FullPath()

		// If endpoint is empty (e.g. 404), use a generic fallback
		if endpoint == "" {
			endpoint = "UNKNOWN"
		}

		APIRequestDuration.WithLabelValues(method, endpoint, status).Observe(float64(duration))
		APIRequestTotal.WithLabelValues(method, endpoint, status).Inc()
	}
}

// PrometheusHandler wraps the standard HTTP handler for Gin.
func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
