package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRequestID(t *testing.T) {
	type existingContextKey struct{}

	ctx := context.WithValue(
		context.Background(),
		existingContextKey{},
		"preserved",
	)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(ctx)
	request.Header.Set("X-Request-ID", "client-supplied-id")

	recorder := httptest.NewRecorder()
	called := false
	var contextID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		contextID = requestIDFromContext(r.Context())

		if got := r.Context().Value(existingContextKey{}); got != "preserved" {
			t.Errorf("existing context value = %v, want preserved", got)
		}

		cancel()
		if err := r.Context().Err(); err != context.Canceled {
			t.Errorf("context error = %v, want context.Canceled", err)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	withRequestID(next).ServeHTTP(recorder, request)

	if !called {
		t.Fatal("downstream handler was not called")
	}

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", response.StatusCode)
	}

	responseID := response.Header.Get("X-Request-ID")
	if responseID == "" {
		t.Fatal("response request ID is empty")
	}

	if responseID == "client-supplied-id" {
		t.Error("middleware trusted the client-supplied request ID")
	}

	if contextID != responseID {
		t.Errorf("context ID = %q, response ID = %q", contextID, responseID)
	}
}

func TestRequestIDFromContextMissing(t *testing.T) {
	got := requestIDFromContext(context.Background())
	if got != "" {
		t.Errorf("request ID = %q, want empty string", got)
	}
}
