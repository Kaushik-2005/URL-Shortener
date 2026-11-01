package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type failingClickRecorder struct {
	err   error
	calls int
	urlID int64
}

func (r *failingClickRecorder) recordClick(
	ctx context.Context,
	urlID int64,
	at time.Time,
) error {
	r.calls++
	r.urlID = urlID
	return r.err
}

func TestRedirectHandlerClickCounting(t *testing.T) {
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		method     string
		code       string
		expiresAt  *time.Time
		requests   int
		wantStatus int
		wantClicks int64
	}{
		{
			name:       "repeated GET",
			method:     http.MethodGet,
			code:       "tracked-guide",
			requests:   3,
			wantStatus: http.StatusFound,
			wantClicks: 3,
		},
		{
			name:       "HEAD",
			method:     http.MethodHead,
			code:       "tracked-guide",
			requests:   1,
			wantStatus: http.StatusFound,
		},
		{
			name:       "expired GET",
			method:     http.MethodGet,
			code:       "tracked-guide",
			expiresAt:  &past,
			requests:   1,
			wantStatus: http.StatusGone,
		},
		{
			name:       "missing GET",
			method:     http.MethodGet,
			code:       "missing",
			requests:   1,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			ctx := context.Background()

			input := urlRecord{
				Code:        "tracked-guide",
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

			handler := newRedirectHandler(store, store)

			for i := 0; i < tt.requests; i++ {
				request := httptest.NewRequest(tt.method, "/"+tt.code, nil)
				request.SetPathValue("code", tt.code)
				recorder := httptest.NewRecorder()

				handler.ServeHTTP(recorder, request)

				if recorder.Code != tt.wantStatus {
					t.Fatalf("request %d status = %d, want %d; body = %s",
						i+1, recorder.Code, tt.wantStatus, recorder.Body.String())
				}

				wantLocation := ""
				if tt.wantStatus == http.StatusFound {
					wantLocation = input.OriginalURL
				}
				if got := recorder.Header().Get("Location"); got != wantLocation {
					t.Errorf("Location = %q, want %q", got, wantLocation)
				}

				if tt.method == http.MethodHead && recorder.Body.Len() != 0 {
					t.Errorf("HEAD response has a body: %q", recorder.Body.String())
				}
			}

			stats, err := store.getClickStatistics(ctx, record.ID)
			if err != nil {
				t.Fatalf("get statistics: %v", err)
			}
			if stats.Total != tt.wantClicks {
				t.Errorf("clicks = %d, want %d", stats.Total, tt.wantClicks)
			}
		})
	}
}

func TestRedirectHandlerRecordingFailure(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "storage failure",
			err:        errors.New("private recording failure details"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal server error\n",
		},
		{
			name:       "deadline exceeded",
			err:        context.DeadlineExceeded,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "request deadline exceeded\n",
		},
		{
			name:       "record deleted before recording",
			err:        errURLNotFound,
			wantStatus: http.StatusNotFound,
			wantBody:   "404 page not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			ctx := context.Background()

			input := urlRecord{
				Code:        "recording-failure",
				OriginalURL: "https://example.com/guide",
			}
			if err := store.save(ctx, input); err != nil {
				t.Fatalf("save URL: %v", err)
			}

			record, err := store.get(ctx, input.Code)
			if err != nil {
				t.Fatalf("get saved URL: %v", err)
			}

			clicks := &failingClickRecorder{err: tt.err}
			request := httptest.NewRequest(http.MethodGet, "/"+input.Code, nil)
			request.SetPathValue("code", input.Code)

			response := httptest.NewRecorder()
			newRedirectHandler(store, clicks).ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if got := response.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := response.Header().Get("Location"); got != "" {
				t.Errorf("unexpected redirect destination: %q", got)
			}
			if clicks.calls != 1 {
				t.Errorf("recorder calls = %d, want 1", clicks.calls)
			}
			if clicks.urlID != record.ID {
				t.Errorf("recorded ID = %d, want %d", clicks.urlID, record.ID)
			}
		})
	}
}
