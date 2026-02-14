package store

import (
	"context"
	"errors"
	"testing"

	"github.com/Yulian302/lfusys-services-commons/test/mocks"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCreate_RetryOnTransientError(t *testing.T) {
	fake := &mocks.FakeRedis{
		SetNXErrs: []error{
			errors.New("timeout"), // first call fails
			nil,                   // second call succeeds
		},
	}

	store := &RedisStoreImpl{
		client: fake,
	}

	err := store.Create(context.Background(), "key")

	require.NoError(t, err)
	require.Equal(t, 2, fake.SetNXCalls) // retry happened
}

func TestIsStateExists_RetryOnTransientError(t *testing.T) {
	fake := &mocks.FakeRedis{
		GetErrs: []error{
			errors.New("timeout"),
			nil,
		},
		DelErrs: []error{
			nil,
		},
	}

	store := &RedisStoreImpl{
		client: fake,
	}

	exists, err := store.IsStateExists(context.Background(), "key")

	require.NoError(t, err)
	require.True(t, exists)

	require.Equal(t, 2, fake.GetCalls)
}

func TestIsStateExists_RetryDeleteOnTransientError(t *testing.T) {
	fake := &mocks.FakeRedis{
		GetErrs: []error{
			nil,
			nil,
		},
		DelErrs: []error{
			errors.New("timeout"),
			nil,
		},
	}

	store := &RedisStoreImpl{
		client: fake,
	}

	exists, err := store.IsStateExists(context.Background(), "key")

	require.NoError(t, err)
	require.True(t, exists)

	require.Equal(t, 2, fake.DelCalls)
}

func TestIsStateExists_FailureAfterRetries(t *testing.T) {
	fake := &mocks.FakeRedis{
		GetErrs: []error{
			errors.New("timeout"),
			errors.New("network error"),
			errors.New("timeout"),
		},
		DelErrs: []error{
			nil,
			nil,
			nil,
		},
	}

	store := &RedisStoreImpl{
		client: fake,
	}

	exists, err := store.IsStateExists(context.Background(), "key")

	require.Error(t, err)
	require.False(t, exists)

	require.Equal(t, 3, fake.GetCalls)
}

func TestIsStateExists_SuccessOnNonTransientErrors(t *testing.T) {
	fake := &mocks.FakeRedis{
		GetErrs: []error{
			redis.Nil,
			redis.Nil,
			redis.Nil,
		},
		DelErrs: []error{
			nil,
			nil,
			nil,
		},
	}

	store := &RedisStoreImpl{
		client: fake,
	}

	exists, err := store.IsStateExists(context.Background(), "key")

	require.NoError(t, err)
	require.False(t, exists)

	require.Equal(t, 1, fake.GetCalls) // errors are not transient, only one call
}
