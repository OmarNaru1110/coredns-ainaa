package ainaa

import (
	"context"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() { plugin.Register(name, setup) }

func setup(c *caddy.Controller) error {
	c.Next()
	if c.NextArg() {
		return plugin.Error(name, c.ArgErr())
	}

	// connect to redis
	redisClient, err := connectRedis(context.Background())
	if err != nil {
		return plugin.Error(name, err)
	}

	// connect to dynamodb
	dynamodbClient, err := connectDynamoDB(context.Background())
	if err != nil {
		return plugin.Error(name, err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return Ainaa{Next: next, redisClient: redisClient, dynamodbClient: dynamodbClient}
	})

	return nil
}
