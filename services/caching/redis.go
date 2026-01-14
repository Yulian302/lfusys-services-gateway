package caching

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCachingService struct {
	client *redis.Client
}

func NewRedisCachingService(c *redis.Client) *RedisCachingService {
	return &RedisCachingService{
		client: c,
	}
}

func (svc *RedisCachingService) Get(ctx context.Context, key string) (string, error) {
	val, err := svc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (svc *RedisCachingService) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	cmd := svc.client.Set(ctx, key, value, ttl)
	return cmd.Err()
}

func (svc *RedisCachingService) Delete(ctx context.Context, key string) error {
	cmd := svc.client.Del(ctx, key)
	return cmd.Err()
}
