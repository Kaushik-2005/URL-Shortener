package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failingLookupRepository struct {
	lookupErr error
}

func (s *failingLookupRepository) save(
	ctx context.Context,
	record urlRecord,
) error {
	return errors.New("save is not supported by this test repository")
}

func (s *failingLookupRepository) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	return urlRecord{}, s.lookupErr
}

func TestLookupHandlersStorageFailure(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		factory func(urlRepository) http.HandlerFunc
	}{
		{
			name: "redirect",
			path: "/abc12345",
			factory: func(repository urlRepository) http.HandlerFunc {
				return newRedirectHandler(repository, newMemoryStore())
			},
		},
		{
			name:    "metadata",
			path:    "/api/v1/urls/abc12345",
			factory: newMetadataHandler,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &failingLookupRepository{
				lookupErr: errors.New("storage unavailable"),
			}
			handler := tt.factory(repository)

			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.SetPathValue("code", "abc12345")

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf(
					"status = %d, want %d",
					recorder.Code, http.StatusInternalServerError,
				)
			}

			body := strings.TrimSpace(recorder.Body.String())
			if body != "internal server error" {
				t.Errorf("body = %q, want internal server error", body)
			}

			if location := recorder.Header().Get("Location"); location != "" {
				t.Errorf("unexpected Location header: %q", location)
			}
		})
	}
}
