package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpenPostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := openPostgres(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	queryCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	var got int
	if err := pool.QueryRow(queryCtx, "SELECT 1").Scan(&got); err != nil {
		t.Fatalf("query PostgreSQL: %v", err)
	}

	if got != 1 {
		t.Errorf("query result = %d, want 1", got)
	}
}
