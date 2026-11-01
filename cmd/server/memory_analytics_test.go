package main

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func TestMemoryStoreClickStatistics(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	input := urlRecord{
		Code:        "analytics-guide",
		OriginalURL: "https://example.com/guide",
	}
	if err := store.save(ctx, input); err != nil {
		t.Fatalf("save URL: %v", err)
	}

	record, err := store.get(ctx, input.Code)
	if err != nil {
		t.Fatalf("get saved URL: %v", err)
	}

	empty, err := store.getClickStatistics(ctx, record.ID)
	if err != nil {
		t.Fatalf("get empty statistics: %v", err)
	}
	if empty.Total != 0 || len(empty.Days) != 0 {
		t.Errorf("initial statistics = %+v, want zero counts", empty)
	}
	if empty.Days == nil {
		t.Error("initial daily results are nil, want an empty slice")
	}

	timestamps := []string{
		"2026-10-31T00:00:00Z",
		"2026-10-30T23:59:59Z",
		"2026-10-31T00:30:00+05:30",
		"2026-10-30T23:30:00-02:00",
	}

	for _, timestamp := range timestamps {
		at, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			t.Fatalf("parse test timestamp: %v", err)
		}
		if err := store.recordClick(ctx, record.ID, at); err != nil {
			t.Fatalf("record click at %s: %v", timestamp, err)
		}
	}

	got, err := store.getClickStatistics(ctx, record.ID)
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}

	wantDays := []dailyClickCount{
		{Date: "2026-10-30", Count: 2},
		{Date: "2026-10-31", Count: 2},
	}

	if got.Total != 4 {
		t.Errorf("total = %d, want 4", got.Total)
	}
	if !slices.Equal(got.Days, wantDays) {
		t.Errorf("daily counts = %+v, want %+v", got.Days, wantDays)
	}
}

func TestMemoryStoreAnalyticsMissingRecord(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	const missingID int64 = 99

	err := store.recordClick(ctx, missingID, at)
	if !errors.Is(err, errURLNotFound) {
		t.Errorf("record click error = %v, want errURLNotFound", err)
	}

	stats, err := store.getClickStatistics(ctx, missingID)
	if !errors.Is(err, errURLNotFound) {
		t.Fatalf("statistics error = %v, want errURLNotFound", err)
	}

	if stats.Total != 0 || stats.Days != nil {
		t.Errorf("statistics = %+v, want zero-value result", stats)
	}
}

func TestMemoryStoreAnalyticsCanceledContext(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	input := urlRecord{
		Code:        "canceled-analytics",
		OriginalURL: "https://example.com/canceled",
	}
	if err := store.save(ctx, input); err != nil {
		t.Fatalf("save URL: %v", err)
	}

	record, err := store.get(ctx, input.Code)
	if err != nil {
		t.Fatalf("get saved URL: %v", err)
	}

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	err = store.recordClick(canceledCtx, record.ID, at)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("record click error = %v, want context.Canceled", err)
	}

	stats, err := store.getClickStatistics(canceledCtx, record.ID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("statistics error = %v, want context.Canceled", err)
	}
	if stats.Total != 0 || stats.Days != nil {
		t.Errorf("failed lookup returned %+v, want zero-value result", stats)
	}

	after, err := store.getClickStatistics(ctx, record.ID)
	if err != nil {
		t.Fatalf("read statistics with active context: %v", err)
	}
	if after.Total != 0 || len(after.Days) != 0 {
		t.Errorf("canceled write changed statistics: %+v", after)
	}
	if after.Days == nil {
		t.Error("existing record returned nil daily results")
	}
}

func TestMemoryStoreAnalyticsCodeReuse(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	const code = "reused-analytics"

	saveAndGet := func(destination string) urlRecord {
		t.Helper()

		input := urlRecord{
			Code:        code,
			OriginalURL: destination,
		}
		if err := store.save(ctx, input); err != nil {
			t.Fatalf("save URL: %v", err)
		}

		record, err := store.get(ctx, code)
		if err != nil {
			t.Fatalf("get saved URL: %v", err)
		}
		return record
	}

	original := saveAndGet("https://example.com/original")

	if err := store.recordClick(ctx, original.ID, at); err != nil {
		t.Fatalf("record original click: %v", err)
	}

	before, err := store.getClickStatistics(ctx, original.ID)
	if err != nil {
		t.Fatalf("get original statistics: %v", err)
	}
	if before.Total != 1 {
		t.Fatalf("original total = %d, want 1", before.Total)
	}

	if err := store.delete(ctx, code); err != nil {
		t.Fatalf("delete original: %v", err)
	}

	replacement := saveAndGet("https://example.com/replacement")
	if replacement.ID == original.ID {
		t.Fatal("replacement reused the original ID")
	}

	err = store.recordClick(ctx, original.ID, at)
	if !errors.Is(err, errURLNotFound) {
		t.Errorf("old-ID recording error = %v, want errURLNotFound", err)
	}

	oldStats, err := store.getClickStatistics(ctx, original.ID)
	if !errors.Is(err, errURLNotFound) {
		t.Fatalf("old-ID statistics error = %v, want errURLNotFound", err)
	}
	if oldStats.Total != 0 || oldStats.Days != nil {
		t.Errorf("old-ID statistics = %+v, want zero-value result", oldStats)
	}

	newStats, err := store.getClickStatistics(ctx, replacement.ID)
	if err != nil {
		t.Fatalf("get replacement statistics: %v", err)
	}
	if newStats.Total != 0 || len(newStats.Days) != 0 {
		t.Errorf("replacement inherited clicks: %+v", newStats)
	}
	if newStats.Days == nil {
		t.Error("replacement returned nil daily results")
	}
}

func TestMemoryStoreRecordClickConcurrent(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	input := urlRecord{
		Code:        "concurrent-clicks",
		OriginalURL: "https://example.com/concurrent",
	}
	if err := store.save(ctx, input); err != nil {
		t.Fatalf("save URL: %v", err)
	}

	record, err := store.get(ctx, input.Code)
	if err != nil {
		t.Fatalf("get saved URL: %v", err)
	}

	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	const workers = 16
	const clicksPerWorker = 25

	start := make(chan struct{})
	results := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			<-start

			for click := 0; click < clicksPerWorker; click++ {
				if err := store.recordClick(ctx, record.ID, at); err != nil {
					results <- err
					return
				}
			}

			results <- nil
		}()
	}
	close(start)

	for i := 0; i < workers; i++ {
		if err := <-results; err != nil {
			t.Errorf("record clicks: %v", err)
		}
	}

	stats, err := store.getClickStatistics(ctx, record.ID)
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}

	const wantTotal int64 = workers * clicksPerWorker
	if stats.Total != wantTotal {
		t.Errorf("total = %d, want %d", stats.Total, wantTotal)
	}

	wantDays := []dailyClickCount{
		{Date: "2026-10-31", Count: wantTotal},
	}
	if !slices.Equal(stats.Days, wantDays) {
		t.Errorf("daily counts = %+v, want %+v", stats.Days, wantDays)
	}
}
