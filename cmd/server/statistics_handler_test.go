package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

type stubStatisticsReader struct {
	calls int
	urlID int64
	err   error
}

func (s *stubStatisticsReader) getClickStatistics(
	ctx context.Context,
	urlID int64,
) (clickStatistics, error) {
	s.calls++
	s.urlID = urlID
	return clickStatistics{}, s.err
}

func TestStatisticsHandler(t *testing.T) {
	type dailyResponse struct {
		Date   string `json:"date"`
		Clicks int64  `json:"clicks"`
	}

	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	firstDay := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	secondDay := firstDay.Add(24 * time.Hour)

	countedDays := []dailyResponse{
		{Date: "2026-10-30", Clicks: 1},
		{Date: "2026-10-31", Clicks: 2},
	}

	tests := []struct {
		name       string
		expiresAt  *time.Time
		seedClicks bool
		wantTotal  int64
		wantDaily  []dailyResponse
	}{
		{
			name:      "no clicks",
			wantDaily: []dailyResponse{},
		},
		{
			name:       "recorded clicks",
			seedClicks: true,
			wantTotal:  3,
			wantDaily:  countedDays,
		},
		{
			name:       "expired link",
			expiresAt:  &past,
			seedClicks: true,
			wantTotal:  3,
			wantDaily:  countedDays,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			ctx := context.Background()

			input := urlRecord{
				Code:        "stats-guide",
				OriginalURL: "https://example.com/guide",
				ExpiresAt:   tt.expiresAt,
			}
			if err := store.save(ctx, input); err != nil {
				t.Fatalf("save URL: %v", err)
			}

			record, err := store.get(ctx, input.Code)
			if err != nil {
				t.Fatalf("get saved URL: %v", err)
			}

			if tt.seedClicks {
				for _, at := range []time.Time{secondDay, firstDay, secondDay} {
					if err := store.recordClick(ctx, record.ID, at); err != nil {
						t.Fatalf("seed click: %v", err)
					}
				}
			}

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/urls/"+input.Code+"/stats",
				nil,
			)
			request.SetPathValue("code", input.Code)

			recorder := httptest.NewRecorder()
			newStatisticsHandler(store, store).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s",
					recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}

			var response struct {
				Code        string          `json:"code"`
				TotalClicks int64           `json:"total_clicks"`
				Daily       []dailyResponse `json:"daily"`
			}

			decoder := json.NewDecoder(recorder.Body)
			if err := decoder.Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				t.Fatalf("response has trailing data: %v", err)
			}

			if response.Code != input.Code {
				t.Errorf("code = %q, want %q", response.Code, input.Code)
			}
			if response.TotalClicks != tt.wantTotal {
				t.Errorf("total = %d, want %d", response.TotalClicks, tt.wantTotal)
			}
			if response.Daily == nil {
				t.Fatal("daily results are nil, want a JSON array")
			}
			if !slices.Equal(response.Daily, tt.wantDaily) {
				t.Errorf("daily counts = %+v, want %+v", response.Daily, tt.wantDaily)
			}
		})
	}
}

func TestStatisticsHandlerMissingURL(t *testing.T) {
	store := newMemoryStore()
	reader := &stubStatisticsReader{}

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/urls/missing/stats",
		nil,
	)
	request.SetPathValue("code", "missing")

	recorder := httptest.NewRecorder()
	newStatisticsHandler(store, reader).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
	if got := recorder.Body.String(); got != "404 page not found\n" {
		t.Errorf("body = %q, want missing-record message", got)
	}
	if reader.calls != 0 {
		t.Errorf("statistics reader calls = %d, want 0", reader.calls)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestStatisticsHandlerReaderFailure(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "storage failure",
			err:        errors.New("private analytics storage details"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal server error\n",
		},
		{
			name:       "deadline exceeded",
			err:        context.DeadlineExceeded,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "request deadline exceeded\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			ctx := context.Background()

			input := urlRecord{
				Code:        "stats-failure",
				OriginalURL: "https://example.com/guide",
			}
			if err := store.save(ctx, input); err != nil {
				t.Fatalf("save URL: %v", err)
			}

			record, err := store.get(ctx, input.Code)
			if err != nil {
				t.Fatalf("get saved URL: %v", err)
			}

			reader := &stubStatisticsReader{err: tt.err}
			request := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/urls/"+input.Code+"/stats",
				nil,
			)
			request.SetPathValue("code", input.Code)

			recorder := httptest.NewRecorder()
			newStatisticsHandler(store, reader).ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if got := recorder.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if reader.calls != 1 {
				t.Errorf("reader calls = %d, want 1", reader.calls)
			}
			if reader.urlID != record.ID {
				t.Errorf("reader URL ID = %d, want %d", reader.urlID, record.ID)
			}
		})
	}
}
