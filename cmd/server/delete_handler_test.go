package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeleteURLHandler(t *testing.T) {
	past := time.Now().Add(-time.Hour)

	tests := []struct {
		name       string
		seedTarget bool
		expiresAt  *time.Time
		wantStatus int
	}{
		{
			name:       "active record",
			seedTarget: true,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "expired record",
			seedTarget: true,
			expiresAt:  &past,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "missing record",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			ctx := context.Background()

			other := urlRecord{
				Code:        "keep-me",
				OriginalURL: "https://example.com/other",
			}
			if err := store.save(ctx, other); err != nil {
				t.Fatalf("save other record: %v", err)
			}

			if tt.seedTarget {
				err := store.save(ctx, urlRecord{
					Code:        "delete-me",
					OriginalURL: "https://example.com/target",
					ExpiresAt:   tt.expiresAt,
				})
				if err != nil {
					t.Fatalf("save target: %v", err)
				}
			}

			request := httptest.NewRequest(
				http.MethodDelete,
				"/api/v1/urls/delete-me",
				nil,
			)
			request.SetPathValue("code", "delete-me")

			recorder := httptest.NewRecorder()
			newDeleteURLHandler(store).ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusNoContent && recorder.Body.Len() != 0 {
				t.Errorf("204 response has a body: %q", recorder.Body.String())
			}

			if _, err := store.get(ctx, "delete-me"); !errors.Is(err, errURLNotFound) {
				t.Errorf("target lookup error = %v, want errURLNotFound", err)
			}

			savedOther, err := store.get(ctx, other.Code)
			if err != nil {
				t.Fatalf("get other record: %v", err)
			}
			if savedOther != other {
				t.Errorf("other record = %+v, want %+v", savedOther, other)
			}
		})
	}
}

type failingDeleter struct {
	deleteErr error
	calls     int
	code      string
}

func (s *failingDeleter) delete(
	ctx context.Context,
	code string,
) error {
	s.calls++
	s.code = code
	return s.deleteErr
}

func TestDeleteURLHandlerStorageFailure(t *testing.T) {
	store := &failingDeleter{
		deleteErr: errors.New("storage unavailable"),
	}

	request := httptest.NewRequest(
		http.MethodDelete,
		"/api/v1/urls/go-guide",
		nil,
	)
	request.SetPathValue("code", "go-guide")

	recorder := httptest.NewRecorder()
	newDeleteURLHandler(store).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf(
			"status = %d, want %d",
			recorder.Code, http.StatusInternalServerError,
		)
	}

	if recorder.Body.String() != "internal server error\n" {
		t.Errorf("unexpected error body: %q", recorder.Body.String())
	}

	if store.calls != 1 {
		t.Errorf("delete calls = %d, want 1", store.calls)
	}

	if store.code != "go-guide" {
		t.Errorf("deleted code = %q, want go-guide", store.code)
	}
}
