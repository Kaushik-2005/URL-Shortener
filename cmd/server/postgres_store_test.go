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

	before, err := store.get(ctx, original.Code)
	if err != nil {
		t.Fatalf("get original before collision: %v", err)
	}
	if before.ID <= 0 {
		t.Fatalf("original ID = %d, want positive ID", before.ID)
	}

	wantStored := original
	wantStored.ID = before.ID

	if before != wantStored {
		t.Fatalf("stored record = %+v, want %+v", before, wantStored)
	}

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

	if got != before {
		t.Errorf("record after collision = %+v, want %+v", got, before)
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

	if got.ID <= 0 {
		t.Fatalf("stored ID = %d, want positive ID", got.ID)
	}

	wantStored := record
	wantStored.ID = got.ID

	if got != wantStored {
		t.Errorf("stored record = %+v, want %+v", got, wantStored)
	}

}

func TestPostgresStoreExpiryRoundTripIntegration(t *testing.T) {
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

	deadline := time.Date(
		2030, time.January, 1,
		12, 0, 0, 123456000,
		time.FixedZone("UTC+0530", 5*3600+30*60),
	)

	original := urlRecord{
		Code:        "expiry_" + suffix,
		OriginalURL: "https://example.com/expiry",
		ExpiresAt:   &deadline,
	}
	store := &postgresStore{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.save(ctx, original); err != nil {
		t.Fatalf("save expiring record: %v", err)
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

	got, err := store.get(ctx, original.Code)
	if err != nil {
		t.Fatalf("get expiring record: %v", err)
	}

	if got.Code != original.Code || got.OriginalURL != original.OriginalURL {
		t.Errorf("record = %+v, want matching code and original URL", got)
	}

	if got.ExpiresAt == nil {
		t.Fatal("expiry is nil, want stored timestamp")
	}

	if !got.ExpiresAt.Equal(deadline) {
		t.Errorf("expiry = %v, want instant %v", *got.ExpiresAt, deadline)
	}
}

func TestPostgresStoreDeleteIntegration(t *testing.T) {
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

	store := &postgresStore{pool: pool}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	saveRecord := func(record urlRecord) {
		t.Helper()

		if err := store.save(ctx, record); err != nil {
			t.Fatalf("save test record: %v", err)
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
				record.Code,
				record.OriginalURL,
			)
			if err != nil {
				t.Errorf("clean up test record: %v", err)
			}
		})
	}

	target := urlRecord{
		Code:        "delete_" + suffix,
		OriginalURL: "https://example.com/target",
	}
	other := urlRecord{
		Code:        "keep_" + suffix,
		OriginalURL: "https://example.com/other",
	}

	saveRecord(target)
	saveRecord(other)

	targetBefore, err := store.get(ctx, target.Code)
	if err != nil {
		t.Fatalf("get target before deletion: %v", err)
	}
	if targetBefore.ID <= 0 {
		t.Fatalf("target ID = %d, want positive ID", targetBefore.ID)
	}

	otherBefore, err := store.get(ctx, other.Code)
	if err != nil {
		t.Fatalf("get other before deletion: %v", err)
	}
	if otherBefore.ID <= 0 {
		t.Fatalf("other ID = %d, want positive ID", otherBefore.ID)
	}

	if err := store.delete(ctx, target.Code); err != nil {
		t.Fatalf("delete target: %v", err)
	}

	got, err := store.get(ctx, target.Code)
	if !errors.Is(err, errURLNotFound) {
		t.Fatalf("get deleted record error = %v, want errURLNotFound", err)
	}
	if got != (urlRecord{}) {
		t.Errorf("deleted record = %+v, want empty record", got)
	}

	if err := store.delete(ctx, target.Code); !errors.Is(err, errURLNotFound) {
		t.Errorf("second delete error = %v, want errURLNotFound", err)
	}

	savedOther, err := store.get(ctx, other.Code)
	if err != nil {
		t.Fatalf("get other record: %v", err)
	}
	if savedOther != otherBefore {
		t.Errorf("other record = %+v, want %+v", savedOther, otherBefore)
	}

	replacement := urlRecord{
		Code:        target.Code,
		OriginalURL: "https://example.com/replacement",
	}
	saveRecord(replacement)

	savedReplacement, err := store.get(ctx, replacement.Code)
	if err != nil {
		t.Fatalf("get replacement: %v", err)
	}
	if savedReplacement.ID <= 0 {
		t.Fatalf("replacement ID = %d, want positive ID", savedReplacement.ID)
	}
	if savedReplacement.ID == targetBefore.ID {
		t.Errorf("replacement reused deleted ID %d", targetBefore.ID)
	}

	wantReplacement := replacement
	wantReplacement.ID = savedReplacement.ID

	if savedReplacement != wantReplacement {
		t.Errorf(
			"replacement = %+v, want %+v",
			savedReplacement,
			wantReplacement,
		)
	}
}
