package metrics

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisMetricsHook implements redis.Hook to collect cache hits and misses.
type RedisMetricsHook struct{}

func (RedisMetricsHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (RedisMetricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		// Only instrument GET requests for hit/miss ratio
		if cmd.Name() == "get" {
			if err == redis.Nil {
				RedisCacheMisses.WithLabelValues("general_cache").Inc()
			} else if err == nil {
				RedisCacheHits.WithLabelValues("general_cache").Inc()
			}
		}
		return err
	}
}

func (RedisMetricsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
