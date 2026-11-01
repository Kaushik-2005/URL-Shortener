package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestRedisCacheRoundTripIntegration(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}

	client, err := newRedisClient(redisURL)
	if err != nil {
		t.Fatalf("configure Redis: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Redis: %v", err)
		}
	})

	cache := &redisCache{client: client}
	suffix, err := generateCode()
	if err != nil {
		t.Fatalf("generate test code: %v", err)
	}

	deadline := time.Date(
		2030, time.January, 1, 12, 0, 0, 123456000, time.UTC,
	)
	record := urlRecord{
		ID:          42,
		Code:        "cache_" + suffix,
		OriginalURL: "https://example.com/cache-test",
		ExpiresAt:   &deadline,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	assertMiss := func() {
		t.Helper()

		got, err := cache.get(ctx, record.Code)
		if !errors.Is(err, errCacheMiss) {
			t.Fatalf("get error = %v, want errCacheMiss", err)
		}
		if got != (urlRecord{}) {
			t.Errorf("miss returned record = %+v, want empty record", got)
		}
	}

	assertMiss()

	if err := cache.set(ctx, record, time.Minute); err != nil {
		t.Fatalf("set cache: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		defer cancel()

		if err := cache.delete(cleanupCtx, record.Code); err != nil {
			t.Errorf("clean up cache key: %v", err)
		}
	})

	got, err := cache.get(ctx, record.Code)
	if err != nil {
		t.Fatalf("get cached record: %v", err)
	}

	if got.ID != record.ID {
		t.Errorf("cached ID = %d, want %d", got.ID, record.ID)
	}
	if got.Code != record.Code || got.OriginalURL != record.OriginalURL {
		t.Errorf("cached record = %+v, want matching code and URL", got)
	}
	if got.ExpiresAt == nil {
		t.Fatal("cached record lost its expiry")
	}
	if !got.ExpiresAt.Equal(deadline) {
		t.Errorf("cached expiry = %v, want %v", *got.ExpiresAt, deadline)
	}

	ttl, err := client.PTTL(ctx, cache.key(record.Code)).Result()
	if err != nil {
		t.Fatalf("read cache TTL: %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Errorf("TTL = %v, want positive and at most one minute", ttl)
	}

	if err := cache.delete(ctx, record.Code); err != nil {
		t.Fatalf("delete cached record: %v", err)
	}

	assertMiss()

	if err := cache.delete(ctx, record.Code); err != nil {
		t.Errorf("delete absent cache key: %v", err)
	}

	for _, ttl := range []time.Duration{
		0,
		-time.Second,
		500 * time.Microsecond,
	} {
		if err := cache.set(ctx, record, ttl); err != nil {
			t.Fatalf("set with skipped TTL %v: %v", ttl, err)
		}

		assertMiss()
	}

}

func TestRedisCacheNamespaceIntegration(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}

	client, err := newRedisClient(redisURL)
	if err != nil {
		t.Fatalf("configure Redis: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Redis: %v", err)
		}
	})

	suffix, err := generateCode()
	if err != nil {
		t.Fatalf("generate namespace suffix: %v", err)
	}

	first := &redisCache{client: client, namespace: "first_" + suffix}
	second := &redisCache{client: client, namespace: "second_" + suffix}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setRecord := func(cache *redisCache, record urlRecord) {
		t.Helper()

		if err := cache.set(ctx, record, time.Minute); err != nil {
			t.Fatalf("set namespaced record: %v", err)
		}

		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(
				context.Background(),
				2*time.Second,
			)
			defer cancel()

			if err := cache.delete(cleanupCtx, record.Code); err != nil {
				t.Errorf("clean up namespaced key: %v", err)
			}
		})
	}

	firstRecord := urlRecord{
		Code:        "go-guide",
		OriginalURL: "https://example.com/first",
	}
	secondRecord := urlRecord{
		Code:        "go-guide",
		OriginalURL: "https://example.com/second",
	}

	setRecord(first, firstRecord)
	setRecord(second, secondRecord)

	gotFirst, err := first.get(ctx, firstRecord.Code)
	if err != nil {
		t.Fatalf("get first namespace: %v", err)
	}
	if gotFirst != firstRecord {
		t.Errorf("first record = %+v, want %+v", gotFirst, firstRecord)
	}

	gotSecond, err := second.get(ctx, secondRecord.Code)
	if err != nil {
		t.Fatalf("get second namespace: %v", err)
	}
	if gotSecond != secondRecord {
		t.Errorf("second record = %+v, want %+v", gotSecond, secondRecord)
	}

	if err := first.delete(ctx, firstRecord.Code); err != nil {
		t.Fatalf("delete first namespace: %v", err)
	}

	if _, err := first.get(ctx, firstRecord.Code); !errors.Is(err, errCacheMiss) {
		t.Errorf("first namespace error = %v, want errCacheMiss", err)
	}

	gotSecond, err = second.get(ctx, secondRecord.Code)
	if err != nil {
		t.Fatalf("get second namespace after deletion: %v", err)
	}
	if gotSecond != secondRecord {
		t.Errorf("second record changed: got %+v, want %+v", gotSecond, secondRecord)
	}
}
