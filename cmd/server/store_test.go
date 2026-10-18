package main

import (
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

	if err := store.save(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, exists := store.get(want.Code)
	if !exists {
		t.Fatal("saved record was not found")
	}

	if got != want {
		t.Fatalf("get = %+v, want %+v", got, want)
	}
}

func TestMemoryStoreGetMissing(t *testing.T) {
	store := newMemoryStore()

	_, exists := store.get("missing")
	if exists {
		t.Fatal("missing code was reported as existing")
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

	if err := store.save(original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	err := store.save(duplicate)
	if !errors.Is(err, errCodeExists) {
		t.Fatalf("save duplicate error = %v, want %v", err, errCodeExists)
	}

	got, exists := store.get(original.Code)
	if !exists {
		t.Fatalf("original record disappeared after collision")
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

			err := store.save(record)

			got, exists := store.get(record.Code)
			if !exists {
				results <- fmt.Errorf(
					"get after save = (%+v, %t), want (%+v), true",
					got, exists, record,
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
