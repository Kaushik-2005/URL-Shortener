package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func BenchmarkPostgresLookup(b *testing.B) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		b.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := openPostgres(context.Background(), databaseURL)
	if err != nil {
		b.Fatalf("open PostgreSQL: %v", err)
	}
	b.Cleanup(pool.Close)

	suffix, err := generateCode()
	if err != nil {
		b.Fatalf("generate benchmark code: %v", err)
	}

	input := urlRecord{
		Code:        "bench_lookup_" + suffix,
		OriginalURL: "https://example.com/benchmark",
	}
	store := &postgresStore{pool: pool}

	setupCtx, cancelSetup := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelSetup()

	if err := store.save(setupCtx, input); err != nil {
		b.Fatalf("save benchmark URL: %v", err)
	}

	b.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM public.urls WHERE code = $1 AND original_url = $2",
			input.Code,
			input.OriginalURL,
		)
		if err != nil {
			b.Errorf("clean up benchmark URL: %v", err)
		}
	})

	saved, err := store.get(setupCtx, input.Code)
	cancelSetup()
	if err != nil {
		b.Fatalf("get benchmark URL: %v", err)
	}
	if saved.ID <= 0 {
		b.Fatalf("stored ID = %d, want positive ID", saved.ID)
	}

	want := input
	want.ID = saved.ID

	b.ReportAllocs()

	for b.Loop() {
		queryCtx, cancelQuery := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		got, err := store.get(queryCtx, input.Code)
		cancelQuery()

		if err != nil {
			b.Fatalf("lookup: %v", err)
		}
		if got != want {
			b.Fatalf("record = %+v, want %+v", got, want)
		}
	}
}
