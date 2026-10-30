package main

import (
	"sync"
	"testing"
	"time"
)

func TestCreateLimiterWindow(t *testing.T) {
	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)

	limiter, err := newCreateLimiter(2, time.Minute, start)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{
			name: "first request",
			now:  start,
			want: true,
		},
		{
			name: "second request",
			now:  start,
			want: true,
		},
		{
			name: "limit reached",
			now:  start,
			want: false,
		},
		{
			name: "just before reset",
			now:  start.Add(time.Minute - time.Nanosecond),
			want: false,
		},
		{
			name: "exactly at reset",
			now:  start.Add(time.Minute),
			want: true,
		},
		{
			name: "second request after reset",
			now:  start.Add(time.Minute),
			want: true,
		},
		{
			name: "new window limit reached",
			now:  start.Add(time.Minute),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := limiter.allow(tt.now); got != tt.want {
				t.Errorf("allow = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestCreateLimiterRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{name: "zero limit", limit: 0, window: time.Minute},
		{name: "negative limit", limit: -1, window: time.Minute},
		{name: "zero window", limit: 1, window: 0},
		{name: "negative window", limit: 1, window: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := newCreateLimiter(tt.limit, tt.window, time.Time{})
			if err == nil {
				t.Fatal("expected configuration error")
			}
			if limiter != nil {
				t.Fatal("expected nil limiter for invalid configuration")
			}
		})
	}
}

func TestCreateLimiterConcurrentAccess(t *testing.T) {
	const limit = 10
	const workers = 100

	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter, err := newCreateLimiter(limit, time.Minute, now)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	start := make(chan struct{})
	results := make(chan bool, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- limiter.allow(now)
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	allowed := 0
	for result := range results {
		if result {
			allowed++
		}
	}

	if allowed != limit {
		t.Errorf("allowed requests = %d, want %d", allowed, limit)
	}
}
