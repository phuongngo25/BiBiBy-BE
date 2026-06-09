package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"nutrix-backend/internal/infrastructure/metrics"
)

func TestPrometheusMiddleware(t *testing.T) {
	// Setup Gin with Middleware
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(metrics.PrometheusMiddleware())
	
	// Dummy route
	r.GET("/api/v1/foods/search", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.JSON(200, gin.H{"status": "ok"})
	})
	
	// Expose metrics route
	r.GET("/metrics", metrics.PrometheusHandler())

	// Simulate hitting the API
	req1, _ := http.NewRequest("GET", "/api/v1/foods/search", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	
	// Fetch metrics
	req2, _ := http.NewRequest("GET", "/metrics", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	
	// Check output
	body := w2.Body.String()
	if !strings.Contains(body, "api_request_total") {
		t.Errorf("Expected metrics to contain api_request_total, but got: %s", body)
	}
	if !strings.Contains(body, `endpoint="/api/v1/foods/search"`) {
		t.Errorf("Expected metrics to contain the endpoint label, but got: %s", body)
	}
	
	t.Log("Successfully collected metrics:")
	
	// Print a small snippet of the metrics output to prove it works
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "api_request_total{") {
			t.Log(line)
		}
	}
}
