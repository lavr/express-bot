package token

import (
	"context"
	"time"
)

// Cache is the interface for token caching.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, token string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// NoopCache is a cache that does nothing (no caching).
type NoopCache struct{}

func (NoopCache) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (NoopCache) Set(ctx context.Context, key string, token string, ttl time.Duration) error {
	return nil
}

func (NoopCache) Delete(ctx context.Context, key string) error {
	return nil
}
