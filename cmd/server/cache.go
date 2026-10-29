package main

import (
	"context"
	"errors"
	"time"
)

var errCacheMiss = errors.New("URL not found in cache")

type urlCache interface {
	get(ctx context.Context, code string) (urlRecord, error)
	set(ctx context.Context, record urlRecord, ttl time.Duration) error
	delete(ctx context.Context, code string) error
}
