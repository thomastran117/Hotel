package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"hotel/application/environment"
)

// Connect creates a Redis client from the given configuration, verifies the
// connection with a PING, and returns the client.
// The caller is responsible for calling client.Close().
func Connect(cfg *environment.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDatabaseIndex,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("cache: ping: %w", err)
	}

	return client, nil
}
