package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedirectHandler(t *testing.T) {
	const destination = "https://example.com/articles/go"

	tests := []struct {
		name         string
		code         string
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "known code",
			code:         "abc12345",
			wantStatus:   http.StatusFound,
			wantLocation: destination,
		},
		{
			name:       "unknown code",
			code:       "missing",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			if err := store.save(context.Background(), urlRecord{
				Code:        "abc12345",
				OriginalURL: destination,
			}); err != nil {
				t.Fatalf("save test record: %v", err)
			}

			handler := newRedirectHandler(store)
			request := httptest.NewRequest(
				http.MethodGet,
				"/"+tt.code,
				nil,
			)
			request.SetPathValue("code", tt.code)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d",
					recorder.Code, tt.wantStatus,
				)
			}

			if got := recorder.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf(
					"Location = %q, want %q",
					got, tt.wantLocation,
				)
			}
		})
	}
}
