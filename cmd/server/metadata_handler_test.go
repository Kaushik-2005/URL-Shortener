package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
			if err := store.save(urlRecord{
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
