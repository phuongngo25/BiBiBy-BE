package rule_engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"nutrix-backend/internal/domain"
)

// Cache Layer constants
const (
	rulesCacheTTL     = 10 * time.Minute
	rulesCacheTimeout = 100 * time.Millisecond
)

// RuleCache defines the Redis interface for storing AggregatedRuleSets.
type RuleCache interface {
	GetRules(ctx context.Context, userID uuid.UUID) (*domain.AggregatedRuleSet, error)
	SetRules(ctx context.Context, userID uuid.UUID, rules domain.AggregatedRuleSet) error
	InvalidateRules(ctx context.Context, userID uuid.UUID) error
}

type redisRuleCache struct {
	client *redis.Client
}

// NewRedisRuleCache initializes the cache with pragmatic fail-open timeout bounds.
func NewRedisRuleCache(client *redis.Client) RuleCache {
	return &redisRuleCache{
		client: client,
	}
}

func (c *redisRuleCache) cacheKey(userID uuid.UUID) string {
	return fmt.Sprintf("user_rules:%s", userID.String())
}

// GetRules attempts to retrieve the AggregatedRuleSet from Redis.
// It uses a strict 100ms timeout context to ensure high-performance graceful degradation.
func (c *redisRuleCache) GetRules(ctx context.Context, userID uuid.UUID) (*domain.AggregatedRuleSet, error) {
	// Fail-open: Bound the Redis fetch to 100ms.
	timeoutCtx, cancel := context.WithTimeout(ctx, rulesCacheTimeout)
	defer cancel()

	key := c.cacheKey(userID)
	val, err := c.client.Get(timeoutCtx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		// Log error but do NOT fail the request. Return nil to trigger DB fallback.
		// log.Printf("[RuleEngine] Cache GET timeout/error for %s: %v", key, err)
		return nil, nil 
	}

	var rules domain.AggregatedRuleSet
	if err := json.Unmarshal([]byte(val), &rules); err != nil {
		return nil, err
	}

	return &rules, nil
}

// SetRules stores the AggregatedRuleSet into Redis with a 10m TTL.
func (c *redisRuleCache) SetRules(ctx context.Context, userID uuid.UUID, rules domain.AggregatedRuleSet) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, rulesCacheTimeout)
	defer cancel()

	bytes, err := json.Marshal(rules)
	if err != nil {
		return err
	}

	key := c.cacheKey(userID)
	return c.client.Set(timeoutCtx, key, bytes, rulesCacheTTL).Err()
}

// InvalidateRules explicitly clears the cache upon Admin/User constraint updates.
func (c *redisRuleCache) InvalidateRules(ctx context.Context, userID uuid.UUID) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, rulesCacheTimeout)
	defer cancel()

	key := c.cacheKey(userID)
	return c.client.Del(timeoutCtx, key).Err()
}
