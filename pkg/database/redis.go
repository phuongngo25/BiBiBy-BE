package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"nutrix-backend/config"
)

// NewRedisClient initializes a singleton Redis client with production-grade timeouts and retry logic.
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:         cfg.RedisURL,
		Password:     cfg.RedisPassword,
		DB:           0, // Use default DB
		DialTimeout:  5 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		MaxRetries:   2,
	}

	rdb := redis.NewClient(opts)

	// Fail-fast startup check: Wait up to 2s for a PING
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", cfg.RedisURL, err)
	}

	log.Printf("[Database] Successfully connected to Redis at %s", cfg.RedisURL)
	return rdb, nil
}
