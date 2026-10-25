package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPostgresStoreSaveLookupAndCollisionIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := openPostgres(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	suffix, err := generateCode()
	if err != nil {
		t.Fatalf("generate test code: %v", err)
	}

	original := urlRecord{
		Code:        "test_" + suffix,
		OriginalURL: "https://example.com/original",
	}

	store := &postgresStore{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.save(ctx, original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM public.urls WHERE code = $1 AND original_url = $2",
			original.Code,
			original.OriginalURL,
		)
		if err != nil {
			t.Errorf("clean up test record: %v", err)
		}
	})

	duplicate := urlRecord{
		Code:        original.Code,
		OriginalURL: "https://example.com/replacement",
	}

	if err := store.save(ctx, duplicate); !errors.Is(err, errCodeExists) {
		t.Fatalf("save duplicate error = %v, want errCodeExists", err)
	}

	got, err := store.get(ctx, original.Code)
	if err != nil {
		t.Fatalf("get original: %v", err)
	}

	if got != original {
		t.Errorf("stored record = %+v, want %+v", got, original)
	}

	missing, err := store.get(ctx, "missing_"+suffix)
	if !errors.Is(err, errURLNotFound) {
		t.Fatalf("get missing error = %v, want errURLNotFound", err)
	}

	if missing != (urlRecord{}) {
		t.Errorf("missing record = %+v, want empty record", missing)
	}
}

func TestPostgresStoreConcurrentInsertIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := openPostgres(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	suffix, err := generateCode()
	if err != nil {
		t.Fatalf("generate test code: %v", err)
	}

	record := urlRecord{
		Code:        "concurrent_" + suffix,
		OriginalURL: "https://example.com/concurrent",
	}
	store := &postgresStore{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const workers = 16
	start := make(chan struct{})
	results := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			<-start
			results <- store.save(ctx, record)
		}()
	}

	close(start)

	successes := 0
	collisions := 0

	for i := 0; i < workers; i++ {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, errCodeExists):
			collisions++
		default:
			t.Errorf("unexpected save error: %v", err)
		}
	}

	if successes > 0 {
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
			defer cancel()

			_, err := pool.Exec(
				cleanupCtx,
				"DELETE FROM public.urls WHERE code = $1 AND original_url = $2",
				record.Code,
				record.OriginalURL,
			)
			if err != nil {
				t.Errorf("clean up test record: %v", err)
			}
		})
	}

	if successes != 1 {
		t.Errorf("successful inserts = %d, want 1", successes)
	}

	if collisions != workers-1 {
		t.Errorf("collisions = %d, want %d", collisions, workers-1)
	}

	got, err := store.get(ctx, record.Code)
	if err != nil {
		t.Fatalf("get saved record: %v", err)
	}

	if got != record {
		t.Errorf("stored record = %+v, want %+v", got, record)
	}
}
