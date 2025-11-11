package ainaa

import (
	"context"
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
