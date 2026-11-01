package main

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestMemoryStoreSaveAndGet(t *testing.T) {
	store := newMemoryStore()
	input := urlRecord{
		Code:        "abc123",
		OriginalURL: "https://example.com",
	}

	if err := store.save(context.Background(), input); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.get(context.Background(), input.Code)
	if err != nil {
		t.Fatalf("get saved record: %v", err)
	}

	if got.ID <= 0 {
		t.Fatalf("saved ID = %d, want positive ID", got.ID)
	}

	if input.ID != 0 {
		t.Errorf("save changed input ID to %d", input.ID)
	}

	want := input
	want.ID = got.ID

	if got != want {
		t.Errorf("saved record = %+v, want %+v", got, want)
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
	ctx := context.Background()

	original := urlRecord{
		Code:        "abc123",
		OriginalURL: "https://example.com/original",
	}
	duplicate := urlRecord{
		Code:        original.Code,
		OriginalURL: "https://example.com/duplicate",
	}

	if err := store.save(ctx, original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	before, err := store.get(ctx, original.Code)
	if err != nil {
		t.Fatalf("get original before collision: %v", err)
	}
	if before.ID <= 0 {
		t.Fatalf("original ID = %d, want positive ID", before.ID)
	}

	err = store.save(ctx, duplicate)
	if !errors.Is(err, errCodeExists) {
		t.Fatalf("duplicate save error = %v, want errCodeExists", err)
	}

	after, err := store.get(ctx, original.Code)
	if err != nil {
		t.Fatalf("get original after collision: %v", err)
	}

	if after != before {
		t.Errorf("record after collision = %+v, want %+v", after, before)
	}
}

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	store := newMemoryStore()
	input := urlRecord{
		Code:        "shared",
		OriginalURL: "https://example.com",
	}

	type result struct {
		saveErr error
		record  urlRecord
		getErr  error
	}

	const workers = 16
	start := make(chan struct{})
	results := make(chan result, workers)

	for i := 0; i < workers; i++ {
		go func() {
			<-start

			saveErr := store.save(context.Background(), input)
			record, getErr := store.get(context.Background(), input.Code)

			results <- result{
				saveErr: saveErr,
				record:  record,
				getErr:  getErr,
			}
		}()
	}
	close(start)

	successes := 0
	var storedID int64

	for i := 0; i < workers; i++ {
		got := <-results

		switch {
		case got.saveErr == nil:
			successes++
		case errors.Is(got.saveErr, errCodeExists):
			// Another worker saved the record first.
		default:
			t.Errorf("unexpected save error: %v", got.saveErr)
		}

		if got.getErr != nil {
			t.Errorf("get after save: %v", got.getErr)
			continue
		}
		if got.record.ID <= 0 {
			t.Errorf("stored ID = %d, want positive ID", got.record.ID)
			continue
		}

		if storedID == 0 {
			storedID = got.record.ID
		} else if got.record.ID != storedID {
			t.Errorf("stored ID = %d, want %d", got.record.ID, storedID)
		}

		want := input
		want.ID = storedID
		if got.record != want {
			t.Errorf("stored record = %+v, want %+v", got.record, want)
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

	targetBefore, err := store.get(ctx, target.Code)
	if err != nil {
		t.Fatalf("get target before deletion: %v", err)
	}

	otherBefore, err := store.get(ctx, other.Code)
	if err != nil {
		t.Fatalf("get other before deletion: %v", err)
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
	if err := store.save(ctx, replacement); err != nil {
		t.Fatalf("reuse deleted code: %v", err)
	}

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

func TestMemoryStoreAssignsNewIDAfterCodeReuse(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	original := urlRecord{
		Code:        "reused-code",
		OriginalURL: "https://example.com/original",
	}

	if err := store.save(ctx, original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	savedOriginal, err := store.get(ctx, original.Code)
	if err != nil {
		t.Fatalf("get original: %v", err)
	}
	if savedOriginal.ID <= 0 {
		t.Fatalf("original ID = %d, want positive ID", savedOriginal.ID)
	}

	if err := store.delete(ctx, original.Code); err != nil {
		t.Fatalf("delete original: %v", err)
	}

	replacement := urlRecord{
		Code:        original.Code,
		OriginalURL: "https://example.com/replacement",
	}

	if err := store.save(ctx, replacement); err != nil {
		t.Fatalf("save replacement: %v", err)
	}

	savedReplacement, err := store.get(ctx, replacement.Code)
	if err != nil {
		t.Fatalf("get replacement: %v", err)
	}
	if savedReplacement.ID <= 0 {
		t.Fatalf("replacement ID = %d, want positive ID", savedReplacement.ID)
	}
	if savedReplacement.ID == savedOriginal.ID {
		t.Errorf("replacement reused original ID %d", savedOriginal.ID)
	}
	if savedReplacement.OriginalURL != replacement.OriginalURL {
		t.Errorf(
			"replacement URL = %q, want %q",
			savedReplacement.OriginalURL,
			replacement.OriginalURL,
		)
	}
}

func TestMemoryStoreConcurrentIDs(t *testing.T) {
	store := newMemoryStore()

	type result struct {
		code   string
		record urlRecord
		err    error
	}

	const workers = 16
	start := make(chan struct{})
	results := make(chan result, workers)

	for i := 0; i < workers; i++ {
		code := "code-" + strconv.Itoa(i)

		go func() {
			<-start

			input := urlRecord{
				Code:        code,
				OriginalURL: "https://example.com/" + code,
			}

			if err := store.save(context.Background(), input); err != nil {
				results <- result{code: code, err: err}
				return
			}

			record, err := store.get(context.Background(), code)
			results <- result{
				code:   code,
				record: record,
				err:    err,
			}
		}()
	}
	close(start)

	seen := make(map[int64]string)

	for i := 0; i < workers; i++ {
		got := <-results

		if got.err != nil {
			t.Errorf("save or lookup for %q: %v", got.code, got.err)
			continue
		}
		if got.record.ID <= 0 {
			t.Errorf("ID for %q = %d, want positive ID", got.code, got.record.ID)
			continue
		}

		if previous, exists := seen[got.record.ID]; exists {
			t.Errorf(
				"ID %d assigned to both %q and %q",
				got.record.ID,
				previous,
				got.code,
			)
		}
		seen[got.record.ID] = got.code

		want := urlRecord{
			ID:          got.record.ID,
			Code:        got.code,
			OriginalURL: "https://example.com/" + got.code,
		}
		if got.record != want {
			t.Errorf("stored record = %+v, want %+v", got.record, want)
		}
	}

	if len(seen) != workers {
		t.Errorf("distinct IDs = %d, want %d", len(seen), workers)
	}
}
