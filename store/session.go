package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type SessionStore interface {
	Create(ctx context.Context, key string) error
	IsStateExists(ctx context.Context, key string) (bool, error)
}

type RedisStoreImpl struct {
	client *redis.Client
}

func NewRedisStoreImpl(client *redis.Client) *RedisStoreImpl {
	return &RedisStoreImpl{
		client: client,
	}
}

func (s *RedisStoreImpl) Create(ctx context.Context, key string) error {
	cmd := s.client.SetNX(ctx, key, "1", time.Minute)
	return cmd.Err()
}

func (s *RedisStoreImpl) IsStateExists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	_ = s.client.Del(ctx, key).Err()
	return true, nil
}

func (s *RedisStoreImpl) Shutdown(ctx context.Context) error {
	return s.client.Close()
}
