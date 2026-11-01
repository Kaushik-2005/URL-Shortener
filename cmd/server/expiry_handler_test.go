package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLookupHandlersExpiry(t *testing.T) {
	const destination = "https://example.com/guide"

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	handlers := []struct {
		name         string
		path         string
		factory      func(*memoryStore) http.HandlerFunc
		activeStatus int
		location     string
	}{
		{
			name: "redirect",
			path: "/go-guide",
			factory: func(store *memoryStore) http.HandlerFunc {
				return newRedirectHandler(store, store)
			},
			activeStatus: http.StatusFound,
			location:     destination,
		},
		{
			name: "metadata",
			path: "/api/v1/urls/go-guide",
			factory: func(store *memoryStore) http.HandlerFunc {
				return newMetadataHandler(store)
			},
			activeStatus: http.StatusOK,
		},
	}

	states := []struct {
		name      string
		expiresAt *time.Time
		expired   bool
	}{
		{name: "no expiry"},
		{name: "future expiry", expiresAt: &future},
		{name: "past expiry", expiresAt: &past, expired: true},
	}

	for _, handlerCase := range handlers {
		for _, state := range states {
			t.Run(handlerCase.name+"/"+state.name, func(t *testing.T) {
				store := newMemoryStore()
				record := urlRecord{
					Code:        "go-guide",
					OriginalURL: destination,
					ExpiresAt:   state.expiresAt,
				}

				if err := store.save(context.Background(), record); err != nil {
					t.Fatalf("save test record: %v", err)
				}

				request := httptest.NewRequest(
					http.MethodGet,
					handlerCase.path,
					nil,
				)
				request.SetPathValue("code", record.Code)

				recorder := httptest.NewRecorder()
				handlerCase.factory(store).ServeHTTP(recorder, request)

				wantStatus := handlerCase.activeStatus
				wantLocation := handlerCase.location
				if state.expired {
					wantStatus = http.StatusGone
					wantLocation = ""
				}

				if recorder.Code != wantStatus {
					t.Errorf("status = %d, want %d", recorder.Code, wantStatus)
				}

				if got := recorder.Header().Get("Location"); got != wantLocation {
					t.Errorf("Location = %q, want %q", got, wantLocation)
				}

				if state.expired && recorder.Body.String() != "URL has expired\n" {
					t.Errorf("expired response body = %q", recorder.Body.String())
				}

				if _, err := store.get(context.Background(), record.Code); err != nil {
					t.Errorf("record should remain stored: %v", err)
				}
			})
		}
	}
}
