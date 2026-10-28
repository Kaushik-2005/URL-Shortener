package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestMemoryStoreSaveAndGet(t *testing.T) {
	store := newMemoryStore()
	want := urlRecord{
		Code:        "abc123",
		OriginalURL: "https://example.com",
	}

	if err := store.save(context.Background(), want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, lookupErr := store.get(context.Background(), want.Code)
	if lookupErr != nil {
		t.Fatalf("get saved record: %v", lookupErr)
	}

	if got != want {
		t.Fatalf("get = %+v, want %+v", got, want)
	}
}

func TestMemoryStoreGetMissing(t *testing.T) {
	store := newMemoryStore()

	got, err := store.get(context.Background(), "missing")
	if !errors.Is(err, errURLNotFound) {
		t.Fatalf("error = %v, want errURLNotFound", err)
	}

	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}
}

func TestMemoryStoreRejectsDuplicateCode(t *testing.T) {
	store := newMemoryStore()
	original := urlRecord{
		Code:        "abc123",
		OriginalURL: "https://example.com/original",
	}
	duplicate := urlRecord{
		Code:        "abc123",
		OriginalURL: "https://example.com/duplicate",
	}

	if err := store.save(context.Background(), original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	err := store.save(context.Background(), duplicate)
	if !errors.Is(err, errCodeExists) {
		t.Fatalf("save duplicate error = %v, want %v", err, errCodeExists)
	}

	got, lookupErr := store.get(context.Background(), original.Code)
	if lookupErr != nil {
		t.Fatalf("get original after collision: %v", lookupErr)
	}

	if got != original {
		t.Fatalf("record after collision = %+v, want %+v", got, original)
	}
}

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	store := newMemoryStore()
	record := urlRecord{
		Code:        "shared",
		OriginalURL: "https://example.com",
	}

	const workers = 16
	start := make(chan struct{})
	results := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			<-start

			err := store.save(context.Background(), record)

			got, lookupErr := store.get(context.Background(), record.Code)
			if lookupErr != nil || got != record {
				results <- fmt.Errorf(
					"get after save = (%+v, %v), want (%+v, nil)",
					got, lookupErr, record,
				)
				return
			}

			results <- err
		}()
	}
	close(start)

	successes := 0
	for i := 0; i < workers; i++ {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, errCodeExists):
			// Another goroutine saved the record first.
		default:
			t.Errorf("unexpected result: %v", err)
		}
	}

	if successes != 1 {
		t.Errorf("successful saves = %d, want 1", successes)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	target := urlRecord{
		Code:        "delete-me",
		OriginalURL: "https://example.com/target",
	}
	other := urlRecord{
		Code:        "keep-me",
		OriginalURL: "https://example.com/other",
	}

	if err := store.save(ctx, target); err != nil {
		t.Fatalf("save target: %v", err)
	}
	if err := store.save(ctx, other); err != nil {
		t.Fatalf("save other: %v", err)
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
	if savedOther != other {
		t.Errorf("other record = %+v, want %+v", savedOther, other)
	}

	replacement := urlRecord{
		Code:        target.Code,
		OriginalURL: "https://example.com/replacement",
	}
	if err := store.save(ctx, replacement); err != nil {
		t.Fatalf("reuse deleted code: %v", err)
	}

	savedReplacement, err := store.get(ctx, replacement.Code)
	if err != nil {
		t.Fatalf("get replacement: %v", err)
	}
	if savedReplacement != replacement {
		t.Errorf(
			"replacement = %+v, want %+v",
			savedReplacement, replacement,
		)
	}
}
