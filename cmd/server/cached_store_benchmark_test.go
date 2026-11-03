package main

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"testing"
	"time"
)

type benchmarkLookupStore struct {
	urlStore
	lookups int
}

func (s *benchmarkLookupStore) get(ctx context.Context, code string) (urlRecord, error) {
	s.lookups++
	return s.urlStore.get(ctx, code)
}

// BenchmarkCachedStoreLookup measures URL lookup only, without HTTP or click writes.
// Cold means the URL key is absent; database and network connections are warm.
func BenchmarkCachedStoreLookup(b *testing.B) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if databaseURL == "" || redisURL == "" {
		b.Skip("TEST_DATABASE_URL and TEST_REDIS_URL are required")
	}

	for _, mode := range []string{"cold", "warm"} {
		b.Run(mode, func(b *testing.B) {
			pool, err := openPostgres(context.Background(), databaseURL)
			if err != nil {
				b.Fatalf("open PostgreSQL: %v", err)
			}
			b.Cleanup(pool.Close)

			client, err := newRedisClient(redisURL)
			if err != nil {
				b.Fatalf("configure Redis: %v", err)
			}
			b.Cleanup(func() {
				if err := client.Close(); err != nil {
					b.Errorf("close Redis: %v", err)
				}
			})

			primary := &benchmarkLookupStore{urlStore: &postgresStore{pool: pool}}
			cache := &redisCache{client: client, namespace: rand.Text()}
			store := &cachedStore{primary: primary, cache: cache, maxTTL: 5 * time.Minute}
			input := urlRecord{
				Code:        "bench_cached_" + rand.Text(),
				OriginalURL: "https://example.com/cached-benchmark",
			}
			setupCtx, cancelSetup := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelSetup()
			if err := primary.save(setupCtx, input); err != nil {
				b.Fatalf("save fixture: %v", err)
			}
			b.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := cache.delete(ctx, input.Code); err != nil {
					b.Errorf("delete cache fixture: %v", err)
				}
				_, err := pool.Exec(ctx,
					"DELETE FROM public.urls WHERE code = $1 AND original_url = $2",
					input.Code, input.OriginalURL)
				if err != nil {
					b.Errorf("delete database fixture: %v", err)
				}
			})

			want, err := primary.get(setupCtx, input.Code)
			if err != nil {
				b.Fatalf("read fixture: %v", err)
			}
			if want.ID <= 0 {
				b.Fatalf("fixture ID = %d, want positive ID", want.ID)
			}
			// Warm both clients before either mode is measured.
			if err := cache.set(setupCtx, want, 5*time.Minute); err != nil {
				b.Fatalf("warm cache: %v", err)
			}
			if got, err := cache.get(setupCtx, input.Code); err != nil || got != want {
				b.Fatalf("verify cache fixture: record = %+v, error = %v", got, err)
			}
			cancelSetup()
			primary.lookups = 0
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if mode == "cold" {
					b.StopTimer()
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := cache.delete(ctx, input.Code); err != nil {
						cancel()
						b.Fatalf("reset cache: %v", err)
					}
					_, err := cache.get(ctx, input.Code)
					cancel()
					if !errors.Is(err, errCacheMiss) {
						b.Fatalf("cold fixture error = %v, want cache miss", err)
					}
					b.StartTimer()
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				got, err := store.get(ctx, input.Code)
				cancel()
				if err != nil || got != want {
					b.Fatalf("lookup: record = %+v, error = %v, want %+v", got, err, want)
				}

				if mode == "cold" {
					b.StopTimer()
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					cached, err := cache.get(ctx, input.Code)
					cancel()
					if err != nil || cached != want {
						b.Fatalf("cache population: record = %+v, error = %v", cached, err)
					}
					b.StartTimer()
				}
			}
			b.StopTimer()

			wantLookups := 0
			if mode == "cold" {
				wantLookups = b.N
			}
			if primary.lookups != wantLookups {
				b.Fatalf("primary lookups = %d, want %d", primary.lookups, wantLookups)
			}
			b.ReportMetric(float64(primary.lookups)/float64(b.N), "primary/op")
		})
	}
}
