package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestPostgresPoolLimitIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must use a postgres or postgresql URL")
	}
	query := parsed.Query()
	query.Set("pool_max_conns", "2")
	query.Set("pool_min_conns", "0")
	parsed.RawQuery = query.Encode()

	pool, err := openPostgres(context.Background(), parsed.String())
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)
	if got := pool.Config().MaxConns; got != 2 {
		t.Fatalf("pool maximum = %d, want 2", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	first, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire first connection: %v", err)
	}
	defer first.Release()
	second, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire second connection: %v", err)
	}
	defer second.Release()
	if got := pool.Stat().AcquiredConns(); got != 2 {
		t.Fatalf("acquired connections = %d, want 2", got)
	}

	waitCtx, cancelWait := context.WithTimeout(ctx, 100*time.Millisecond)
	third, err := pool.Acquire(waitCtx)
	cancelWait()
	if third != nil {
		third.Release()
		t.Fatal("third acquisition succeeded while both pool connections were held")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("third acquisition error = %v, want deadline exceeded", err)
	}
	if got := pool.Stat().TotalConns(); got > 2 {
		t.Fatalf("total connections = %d, exceeds maximum 2", got)
	}

	first.Release()
	var value int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil {
		t.Fatalf("query after releasing connection: %v", err)
	}
	if value != 1 {
		t.Errorf("query result = %d, want 1", value)
	}
	t.Log("Two held connections blocked a third acquisition until its deadline; a released connection was reusable")
}
