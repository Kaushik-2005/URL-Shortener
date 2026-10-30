package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWithCreateRateLimit(t *testing.T) {
	start := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	now := start

	limiter, err := newCreateLimiter(1, time.Minute, start)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	})

	handler := withCreateRateLimit(
		limiter,
		func() time.Time { return now },
		next,
	)

	send := func() *httptest.ResponseRecorder {
		t.Helper()

		request := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/urls",
			nil,
		)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	first := send()
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", first.Code)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}

	second := send()
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second status = %d, want 429", second.Code)
	}
	if calls != 1 {
		t.Errorf("handler calls after rejection = %d, want 1", calls)
	}
	if got := second.Body.String(); got != "creation rate limit exceeded\n" {
		t.Errorf("rejection body = %q", got)
	}
	if got := second.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	now = start.Add(time.Minute)

	third := send()
	if third.Code != http.StatusCreated {
		t.Errorf("status after reset = %d, want 201", third.Code)
	}
	if calls != 2 {
		t.Errorf("handler calls after reset = %d, want 2", calls)
	}
}
