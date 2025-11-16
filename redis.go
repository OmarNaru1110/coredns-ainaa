package ainaa

import (
	"context"
	"encoding/json"
	"os"

	"github.com/redis/go-redis/v9"
)

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
	return cachedDomains[0], nil
}

func getCachedDomain(ctx context.Context, client *redis.Client, domain string) (CachedDomain, error) {
	if val, err := client.JSONGet(ctx, domain).Result(); err == nil {
		cachedVal, err := parseJsonCachedDomain(val)
		if err != nil {
			return CachedDomain{}, err
		}
		return cachedVal, nil
	}
	return CachedDomain{}, redis.Nil
}
