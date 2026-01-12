package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type SessionStore interface {
	Create(ctx context.Context, prefix, key, value string) error
	Validate(ctx context.Context, prefix, state string) (bool, error)
}

type RedisStoreImpl struct {
	client *redis.Client
}

func NewRedisStoreImpl(client *redis.Client) *RedisStoreImpl {
	return &RedisStoreImpl{
		client: client,
	}
}

func (s *RedisStoreImpl) Create(ctx context.Context, prefix, key, value string) error {
	fullKey := prefix + key
	cmd := s.client.Set(ctx, fullKey, value, time.Minute)
	return cmd.Err()
}

func (s *RedisStoreImpl) Validate(ctx context.Context, prefix, state string) (bool, error) {
	fullKey := prefix + state
	_, err := s.client.Get(ctx, fullKey).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	_ = s.client.Del(ctx, fullKey).Err()
	return true, nil
}
