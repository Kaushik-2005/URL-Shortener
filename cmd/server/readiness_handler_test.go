package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadinessHandler(t *testing.T) {
	storageErr := errors.New("private database failure details")

	tests := []struct {
		name       string
		checkErr   error
		canceled   bool
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ready",
			wantStatus: http.StatusOK,
			wantBody:   "OK\n",
		},
		{
			name:       "storage unavailable",
			checkErr:   storageErr,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "not ready\n",
		},
		{
			name:       "request canceled",
			canceled:   true,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "not ready\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.canceled {
				cancel()
			}

			calls := 0
			check := func(ctx context.Context) error {
				calls++

				deadline, ok := ctx.Deadline()
				if !ok {
					t.Error("check context has no deadline")
				} else if remaining := time.Until(deadline); remaining > time.Second {
					t.Errorf("remaining timeout = %v, want at most 1s", remaining)
				}

				if tt.canceled {
					if !errors.Is(ctx.Err(), context.Canceled) {
						t.Errorf("context error = %v, want context.Canceled", ctx.Err())
					}
					return ctx.Err()
				}

				return tt.checkErr
			}

			request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			request = request.WithContext(parent)
			recorder := httptest.NewRecorder()

			newReadinessHandler(check).ServeHTTP(recorder, request)

			if calls != 1 {
				t.Errorf("check calls = %d, want 1", calls)
			}
			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if got := recorder.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			if strings.Contains(recorder.Body.String(), storageErr.Error()) {
				t.Error("response exposed internal storage error")
			}
		})
	}
}
