package metrics_test

import (
	"context"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	"nutrix-backend/internal/infrastructure/metrics"
)

func TestRedisMetricsHook(t *testing.T) {
	// Setup Redis Mini Mock (or real if available, but we just trigger the hook directly for the test)
	// Actually, we can use miniredis if available, or just mock the hook call.
	// Since we just want to prove the Prometheus Counter increases, we can trigger the hook manually.
	
	hook := metrics.RedisMetricsHook{}
	
	// Create a dummy ProcessHook
	processHook := hook.ProcessHook(func(ctx context.Context, cmd redis.Cmder) error {
		// Simulate a Redis Hit
		return nil
	})
	
	cmd := redis.NewStringCmd(context.Background(), "get", "user:123")
	
	// Call it
	_ = processHook(context.Background(), cmd)
	
	// Check the metric
	expectedMetrics := `
		# HELP redis_cache_hits_total Đếm số lần cache hit thành công trên Redis.
		# TYPE redis_cache_hits_total counter
		redis_cache_hits_total{cache_type="general_cache"} 1
	`
	
	err := testutil.CollectAndCompare(metrics.RedisCacheHits, strings.NewReader(expectedMetrics), "redis_cache_hits_total")
	if err != nil {
		t.Errorf("Metric comparison failed: %v", err)
	} else {
		t.Log("PASS: redis_cache_hits_total successfully collected!")
		t.Log(`redis_cache_hits_total{cache_type="general_cache"} 1`)
	}
}
