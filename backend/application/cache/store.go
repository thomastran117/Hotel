package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound is returned by Get when the key does not exist.
var ErrNotFound = errors.New("cache: key not found")

// Store is the cache abstraction used by application features.
// Callers depend on this interface rather than on *redis.Client directly.
type Store interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// RedisStore implements Store backed by Redis.
type RedisStore struct {
	client *redis.Client
}

// NewStore returns a Store backed by the given Redis client.
func NewStore(client *redis.Client) Store {
	return &RedisStore{client: client}
}

func (s *RedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("cache: set %q: %w", key, err)
	}
	return nil
}

func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("cache: get %q: %w", key, err)
	}
	return val, nil
}

func (s *RedisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("cache: delete %q: %w", key, err)
	}
	return nil
}
