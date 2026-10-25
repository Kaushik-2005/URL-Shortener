package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failingSaveRepository struct {
	saveErr   error
	saveCalls int
}

func (s *failingSaveRepository) save(ctx context.Context, record urlRecord) error {
	s.saveCalls++
	return s.saveErr
}

func (s *failingSaveRepository) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	return urlRecord{}, errURLNotFound
}

func TestCreateURLHandlerSaveFailure(t *testing.T) {
	store := &failingSaveRepository{
		saveErr: errors.New("storage unavailable"),
	}

	generatorCalls := 0
	generate := func() (string, error) {
		generatorCalls++
		return "abc12345", nil
	}

	handler := newCreateURLHandler(
		store,
		"https://127.0.0.1:8080",
		generate,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/urls",
		strings.NewReader(`{"url":"https://example.com"}`),
	)
	defer request.Body.Close()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf(
			"status = %d, want %d",
			recorder.Code, http.StatusInternalServerError,
		)
	}

	if got := strings.TrimSpace(recorder.Body.String()); got != "internal server error" {
		t.Errorf("body = %q, want internal server error", got)
	}

	if store.saveCalls != 1 {
		t.Errorf("save calls = %d, want 1", store.saveCalls)
	}

	if generatorCalls != 1 {
		t.Errorf("generator calls = %d, want 1", generatorCalls)
	}
}
