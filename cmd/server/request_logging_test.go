package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWithRequestLogging(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantRoute  string
		wantStatus int
	}{
		{
			name:       "matched route",
			path:       "/healthz",
			wantRoute:  "GET /healthz",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "unmatched route",
			path:       "/no/such/path",
			wantRoute:  "",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))

			mux := http.NewServeMux()
			mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			handler := withRequestID(
				withRequestTimeout(
					5*time.Second,
					withRequestLogging(logger, mux),
				),
			)

			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()

			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}

			var entry struct {
				Message    string   `json:"msg"`
				Level      string   `json:"level"`
				RequestID  string   `json:"request_id"`
				Method     string   `json:"method"`
				Route      string   `json:"route"`
				DurationMS *float64 `json:"duration_ms"`
			}

			decoder := json.NewDecoder(&output)
			if err := decoder.Decode(&entry); err != nil {
				t.Fatalf("decode log: %v", err)
			}

			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				t.Fatalf("expected one log entry; second decode error = %v", err)
			}

			if entry.Message != "HTTP request completed" {
				t.Errorf("message = %q", entry.Message)
			}
			if entry.Level != "INFO" {
				t.Errorf("level = %q, want INFO", entry.Level)
			}

			responseID := response.Header.Get("X-Request-ID")
			if responseID == "" {
				t.Fatal("response request ID is empty")
			}
			if entry.RequestID != responseID {
				t.Errorf("log request ID = %q, want %q", entry.RequestID, responseID)
			}
			if entry.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", entry.Method)
			}
			if entry.Route != tt.wantRoute {
				t.Errorf("route = %q, want %q", entry.Route, tt.wantRoute)
			}
			if entry.DurationMS == nil {
				t.Fatal("duration_ms field is missing or null")
			}
			if *entry.DurationMS < 0 {
				t.Errorf("duration = %f ms, want nonnegative", *entry.DurationMS)
			}
		})
	}
}
