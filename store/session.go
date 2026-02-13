package store

import (
	"context"
	"time"

	"github.com/Yulian302/lfusys-services-commons/retries"
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
	return retries.Retry(
		ctx,
		retries.DefaultAttempts,
		retries.DefaultBaseDelay,
		func() error {
			cmd := s.client.SetNX(ctx, key, "1", time.Minute)
			return cmd.Err()
		},
		retries.IsRetriableRedisError,
	)
}

func (s *RedisStoreImpl) IsStateExists(ctx context.Context, key string) (bool, error) {
	var exists bool

	err := retries.Retry(
		ctx,
		retries.DefaultAttempts,
		retries.DefaultBaseDelay,
		func() error {
			val, err := s.client.Get(ctx, key).Result()
			if err == redis.Nil {
				exists = false
				return nil
			}

			if err != nil {
				return err
			}

			exists = val != ""
			return s.client.Del(ctx, key).Err()
		},
		retries.IsRetriableRedisError,
	)
	return exists, err
}

func (s *RedisStoreImpl) Shutdown(ctx context.Context) error {
	return s.client.Close()
}
