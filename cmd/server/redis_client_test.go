package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRedisClientIntegration(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL is not set")
	}

	client, err := newRedisClient(redisURL)
	if err != nil {
		t.Fatalf("configure Redis client: %v", err)
	}

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Redis client: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reply, err := client.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	if reply != "PONG" {
		t.Errorf("ping reply = %q, want PONG", reply)
	}
}
