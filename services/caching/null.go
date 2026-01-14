package caching

import (
	"time"
)

type NullCachingService struct {
	// do nothing
}

func NewNullCachingService() *NullCachingService {
	return &NullCachingService{}
}

func (svc *NullCachingService) Get(key string) (string, error) {
	return "", nil
}

func (svc *NullCachingService) Set(key string, value string, ttl time.Duration) error {
	return nil
}

func (svc *NullCachingService) Delete(key string) error {
	return nil
}
