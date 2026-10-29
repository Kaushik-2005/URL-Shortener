package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisURLKeyPrefix = "url-shortener:v1:url:"

type redisCache struct {
	client    *redis.Client
	namespace string
}

func (c *redisCache) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	data, err := c.client.Get(ctx, c.key(code)).Bytes()
	if errors.Is(err, redis.Nil) {
		return urlRecord{}, errCacheMiss
	}
	if err != nil {
		return urlRecord{}, fmt.Errorf("get cached URL: %w", err)
	}

	var record urlRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return urlRecord{}, fmt.Errorf("decode cached URL: %w", err)
	}

	if record.Code != code || validateURL(record.OriginalURL) != nil {
		return urlRecord{}, errors.New("cached URL record is invalid")
	}

	return record, nil
}

func (c *redisCache) set(
	ctx context.Context,
	record urlRecord,
	ttl time.Duration,
) error {
	ttl = ttl.Truncate(time.Millisecond)
	if ttl <= 0 {
		return nil
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode cached URL: %w", err)
	}

	err = c.client.Set(
		ctx,
		c.key(record.Code),
		data,
		ttl,
	).Err()
	if err != nil {
		return fmt.Errorf("set cached URL: %w", err)
	}

	return nil
}

func (c *redisCache) delete(
	ctx context.Context,
	code string,
) error {
	if err := c.client.Del(ctx, c.key(code)).Err(); err != nil {
		return fmt.Errorf("delete cached URL: %w", err)
	}

	return nil
}

var _ urlCache = (*redisCache)(nil)

func (c *redisCache) key(code string) string {
	if c.namespace == "" {
		return redisURLKeyPrefix + code
	}

	return redisURLKeyPrefix + c.namespace + ":" + code
}
