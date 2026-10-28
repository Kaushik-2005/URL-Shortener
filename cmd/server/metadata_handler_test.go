package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetadataHandler(t *testing.T) {
	const destination = "https://example.com/articles/go"

	tests := []struct {
		name       string
		code       string
		wantStatus int
	}{
		{
			name:       "known code",
			code:       "abc12345",
			wantStatus: http.StatusOK,
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

			handler := newMetadataHandler(store)
			request := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/urls/"+tt.code,
				nil,
			)
			request.SetPathValue("code", tt.code)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					recorder.Code, tt.wantStatus, recorder.Body.String(),
				)
			}

			if tt.wantStatus == http.StatusNotFound {
				return
			}

			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}

			var response struct {
				Code string `json:"code"`
				URL  string `json:"url"`
			}

			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if response.Code != tt.code {
				t.Errorf("code = %q, want %q", response.Code, tt.code)
			}

			if response.URL != destination {
				t.Errorf("URL = %q, want %q", response.URL, destination)
			}
		})
	}
}

func TestMetadataHandlerExpiryField(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

	tests := []struct {
		name      string
		expiresAt *time.Time
	}{
		{name: "permanent link"},
		{name: "expiring link", expiresAt: &future},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			record := urlRecord{
				Code:        "go-guide",
				OriginalURL: "https://example.com/guide",
				ExpiresAt:   tt.expiresAt,
			}

			if err := store.save(context.Background(), record); err != nil {
				t.Fatalf("save test record: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/urls/"+record.Code,
				nil,
			)
			request.SetPathValue("code", record.Code)

			recorder := httptest.NewRecorder()
			newMetadataHandler(store).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					recorder.Code, http.StatusOK, recorder.Body.String(),
				)
			}

			var response map[string]json.RawMessage
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode metadata: %v", err)
			}

			rawExpiry, exists := response["expires_at"]

			if tt.expiresAt == nil {
				if exists {
					t.Errorf("expires_at should be omitted, got %s", rawExpiry)
				}
				return
			}

			if !exists {
				t.Fatal("expires_at field is missing")
			}

			var got time.Time
			if err := json.Unmarshal(rawExpiry, &got); err != nil {
				t.Fatalf("decode expiry: %v", err)
			}

			if !got.Equal(*tt.expiresAt) {
				t.Errorf("expiry = %v, want %v", got, *tt.expiresAt)
			}
		})
	}
}
