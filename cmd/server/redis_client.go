package main

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func newRedisClient(redisURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis configuration: %w", err)
	}

	options.DialTimeout = time.Second
	options.ReadTimeout = time.Second
	options.WriteTimeout = time.Second
	options.ContextTimeoutEnabled = true
	options.MaxRetries = -1

	return redis.NewClient(options), nil
}
