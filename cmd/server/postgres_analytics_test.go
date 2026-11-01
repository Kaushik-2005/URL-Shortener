package main

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"
)

func TestPostgresRecordClickIntegration(t *testing.T) {
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

	input := urlRecord{
		Code:        "clicks_" + suffix,
		OriginalURL: "https://example.com/analytics",
	}
	store := &postgresStore{pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := store.save(ctx, input); err != nil {
		t.Fatalf("save URL: %v", err)
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
			input.Code,
			input.OriginalURL,
		)
		if err != nil {
			t.Errorf("clean up URL: %v", err)
		}
	})

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

	for _, date := range []string{"2026-10-30", "2026-10-31"} {
		var count int64
		err := pool.QueryRow(
			ctx,
			`SELECT click_count
			 FROM public.url_daily_clicks
			 WHERE url_id = $1 AND click_date = $2::date`,
			record.ID,
			date,
		).Scan(&count)
		if err != nil {
			t.Fatalf("read count for %s: %v", date, err)
		}
		if count != 2 {
			t.Errorf("count for %s = %d, want 2", date, count)
		}
	}

	var rows int64
	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM public.url_daily_clicks WHERE url_id = $1",
		record.ID,
	).Scan(&rows)
	if err != nil {
		t.Fatalf("count daily rows: %v", err)
	}
	if rows != 2 {
		t.Errorf("daily rows = %d, want 2", rows)
	}

	stats, err := store.getClickStatistics(ctx, record.ID)
	if err != nil {
		t.Fatalf("get recorded statistics: %v", err)
	}

	wantDays := []dailyClickCount{
		{Date: "2026-10-30", Count: 2},
		{Date: "2026-10-31", Count: 2},
	}

	if stats.Total != 4 {
		t.Errorf("total = %d, want 4", stats.Total)
	}
	if !slices.Equal(stats.Days, wantDays) {
		t.Errorf("daily counts = %+v, want %+v", stats.Days, wantDays)
	}

	if err := store.delete(ctx, record.Code); err != nil {
		t.Fatalf("delete URL: %v", err)
	}

	var remaining int64
	err = pool.QueryRow(
		ctx,
		"SELECT COUNT(*) FROM public.url_daily_clicks WHERE url_id = $1",
		record.ID,
	).Scan(&remaining)
	if err != nil {
		t.Fatalf("count remaining daily rows: %v", err)
	}
	if remaining != 0 {
		t.Errorf("daily rows after deletion = %d, want 0", remaining)
	}

	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	err = store.recordClick(ctx, record.ID, at)
	if !errors.Is(err, errURLNotFound) {
		t.Errorf("deleted-ID recording error = %v, want errURLNotFound", err)
	}

	missing, err := store.getClickStatistics(ctx, record.ID)
	if !errors.Is(err, errURLNotFound) {
		t.Fatalf("deleted-ID statistics error = %v, want errURLNotFound", err)
	}
	if missing.Total != 0 || missing.Days != nil {
		t.Errorf("deleted-ID statistics = %+v, want zero-value result", missing)
	}
}

func TestPostgresRecordClickConcurrentIntegration(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var urlID int64
	err = pool.QueryRow(
		ctx,
		`INSERT INTO public.urls (code, original_url)
		 VALUES ($1, $2)
		 RETURNING id`,
		"concurrent_clicks_"+suffix,
		"https://example.com/concurrent-clicks",
	).Scan(&urlID)
	if err != nil {
		t.Fatalf("insert test URL: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM public.urls WHERE id = $1",
			urlID,
		); err != nil {
			t.Errorf("clean up test URL: %v", err)
		}
	})

	store := &postgresStore{pool: pool}
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	const workers = 16
	const clicksPerWorker = 25

	start := make(chan struct{})
	results := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			<-start

			for click := 0; click < clicksPerWorker; click++ {
				if err := store.recordClick(ctx, urlID, at); err != nil {
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

	stats, err := store.getClickStatistics(ctx, urlID)
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
