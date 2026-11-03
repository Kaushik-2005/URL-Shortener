package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkRedirectHandlerMemory(b *testing.B) {
	store := newMemoryStore()
	ctx := context.Background()

	input := urlRecord{
		Code:        "benchmark-guide",
		OriginalURL: "https://example.com/guide",
	}
	if err := store.save(ctx, input); err != nil {
		b.Fatalf("save URL: %v", err)
	}

	record, err := store.get(ctx, input.Code)
	if err != nil {
		b.Fatalf("get saved URL: %v", err)
	}

	handler := newRedirectHandler(store, store)
	request := httptest.NewRequest(http.MethodGet, "/"+input.Code, nil)
	request.SetPathValue("code", input.Code)

	b.ReportAllocs()

	for b.Loop() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusFound {
			b.Fatalf("status = %d, want 302", response.Code)
		}
		if got := response.Header().Get("Location"); got != input.OriginalURL {
			b.Fatalf("Location = %q, want %q", got, input.OriginalURL)
		}
	}

	stats, err := store.getClickStatistics(ctx, record.ID)
	if err != nil {
		b.Fatalf("get statistics: %v", err)
	}
	if stats.Total != int64(b.N) {
		b.Fatalf("clicks = %d, want %d", stats.Total, b.N)
	}
}

func BenchmarkRedirectHandlerMemoryParallel(b *testing.B) {
	store := newMemoryStore()
	ctx := context.Background()

	input := urlRecord{
		Code:        "benchmark-guide",
		OriginalURL: "https://example.com/guide",
	}
	if err := store.save(ctx, input); err != nil {
		b.Fatalf("save URL: %v", err)
	}

	record, err := store.get(ctx, input.Code)
	if err != nil {
		b.Fatalf("get saved URL: %v", err)
	}

	handler := newRedirectHandler(store, store)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		request := httptest.NewRequest(http.MethodGet, "/"+input.Code, nil)
		request.SetPathValue("code", input.Code)

		for pb.Next() {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusFound {
				b.Errorf("status = %d, want 302", response.Code)
				return
			}
			if got := response.Header().Get("Location"); got != input.OriginalURL {
				b.Errorf("Location = %q, want %q", got, input.OriginalURL)
				return
			}
		}
	})

	b.StopTimer()

	stats, err := store.getClickStatistics(ctx, record.ID)
	if err != nil {
		b.Fatalf("get statistics: %v", err)
	}
	if stats.Total != int64(b.N) {
		b.Fatalf("clicks = %d, want %d", stats.Total, b.N)
	}
}
