package main

import (
	"context"
	"crypto/rand"
	"os"
	"testing"
	"time"
)

func BenchmarkRedisWarmLookup(b *testing.B) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		b.Skip("TEST_REDIS_URL is not set")
	}

	client, err := newRedisClient(redisURL)
	if err != nil {
		b.Fatalf("configure Redis: %v", err)
	}
	b.Cleanup(func() {
		if err := client.Close(); err != nil {
			b.Errorf("close Redis: %v", err)
		}
	})

	cache := &redisCache{
		client:    client,
		namespace: rand.Text(),
	}

	suffix, err := generateCode()
	if err != nil {
		b.Fatalf("generate benchmark code: %v", err)
	}

	want := urlRecord{
		ID:          42,
		Code:        "bench_lookup_" + suffix,
		OriginalURL: "https://example.com/benchmark",
	}

	setupCtx, cancelSetup := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelSetup()

	if err := cache.set(setupCtx, want, 5*time.Minute); err != nil {
		b.Fatalf("populate cache: %v", err)
	}

	b.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		defer cancel()

		if err := cache.delete(cleanupCtx, want.Code); err != nil {
			b.Errorf("clean up cache entry: %v", err)
		}
	})

	warm, err := cache.get(setupCtx, want.Code)
	cancelSetup()
	if err != nil {
		b.Fatalf("verify warm entry: %v", err)
	}
	if warm != want {
		b.Fatalf("warm record = %+v, want %+v", warm, want)
	}

	b.ReportAllocs()

	for b.Loop() {
		queryCtx, cancelQuery := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		got, err := cache.get(queryCtx, want.Code)
		cancelQuery()

		if err != nil {
			b.Fatalf("cached lookup: %v", err)
		}
		if got != want {
			b.Fatalf("record = %+v, want %+v", got, want)
		}
	}
}
