package ainaa

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

// RedisRepository implements CacheRepository using Redis.
type RedisRepository struct {
	client *redis.Client
}

// NewRedisRepository creates a new RedisRepository.
func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{client: client}
}

func connectRedis(ctx context.Context) (*redis.Client, error) {
	addr := os.Getenv("REDIS_ADDR")
	password := os.Getenv("REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
		Protocol: 2,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

func parseJsonCachedDomain(data string) (CachedDomain, error) {
	var cachedDomains []CachedDomain
	err := json.Unmarshal([]byte(data), &cachedDomains)
	if err != nil {
		return CachedDomain{}, err
	}
	if len(cachedDomains) == 0 {
		return CachedDomain{}, fmt.Errorf("empty cached domain list")
	}
	return cachedDomains[0], nil
}

// Get retrieves a domain from the cache.
func (r *RedisRepository) Get(ctx context.Context, domain string) (CachedDomain, error) {
	val, err := r.client.JSONGet(ctx, domain, "$").Result()
	if err != nil {
		return CachedDomain{}, err // Return the actual error
	}

	cachedVal, err := parseJsonCachedDomain(val)
	if err != nil {
		return CachedDomain{}, err
	}

	return cachedVal, nil
}

// Set stores a domain in the cache.
func (r *RedisRepository) Set(ctx context.Context, domain string, value CachedDomain) error {
	return r.client.JSONSet(ctx, domain, "$", value).Err()
}
