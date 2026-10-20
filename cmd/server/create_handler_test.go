package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateURLHandlerSuccess(t *testing.T) {
	store := newMemoryStore()
	baseURL := "http://127.0.0.1:8080"
	handler := newCreateURLHandler(store, baseURL, generateCode)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/urls",
		strings.NewReader(`{"url":"https://example.com/articles/go"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	defer request.Body.Close()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code, http.StatusCreated, recorder.Body.String(),
		)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var response struct {
		Code     string `json:"code"`
		URL      string `json:"url"`
		ShortURL string `json:"short_url"`
	}

	decoder := json.NewDecoder(recorder.Body)

	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("expected response to end after one JSON value; got error %v", err)
	}

	if len(response.Code) != 8 {
		t.Fatalf("code length = %d, want 8", len(response.Code))
	}

	wantURL := "https://example.com/articles/go"
	if response.URL != wantURL {
		t.Errorf("URL = %q, want %q", response.URL, wantURL)
	}

	wantShortURL := baseURL + "/" + response.Code
	if response.ShortURL != wantShortURL {
		t.Errorf("short URL = %q, want %q", response.ShortURL, wantShortURL)
	}

	record, exists := store.get(response.Code)
	if !exists {
		t.Fatal("created record was not saved")
	}

	if record.Code != response.Code || record.OriginalURL != wantURL {
		t.Errorf("saved record = %+v, does not match created URL", record)
	}
}

func TestCreateURLHandlerRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "malformed JSON",
			body:       `{"url":"https://example.com}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing URL",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unsupported scheme",
			body:       `{"url":"ftp://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing hostname",
			body:       `{"url":"https:///articles/go"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown field",
			body:       `{"url":"https://example.com","admin":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "second JSON value",
			body:       `{"url":"https://example.com"} {}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "oversized body",
			body:       `{"url":"https://example.com/` + strings.Repeat("x", 4096) + `"}`,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			handler := newCreateURLHandler(
				store,
				"http://127.0.0.1:8080",
				generateCode,
			)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/urls",
				strings.NewReader(tt.body),
			)
			request.Header.Set("Content-Type", "application/json")
			defer request.Body.Close()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body = %s",
					recorder.Code, tt.wantStatus, recorder.Body.String(),
				)
			}

			store.mu.RLock()
			count := len(store.urls)
			store.mu.RUnlock()

			if count != 0 {
				t.Errorf("stored records = %d, want 0", count)
			}
		})
	}
}

func TestCreateURLHandlerRetriesCollision(t *testing.T) {
	store := newMemoryStore()
	original := urlRecord{
		Code:        "taken123",
		OriginalURL: "https://example.com/original",
	}

	if err := store.save(original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	calls := 0
	generate := func() (string, error) {
		calls++
		if calls == 1 {
			return original.Code, nil
		}
		return "fresh456", nil
	}

	handler := newCreateURLHandler(
		store,
		"http://127.0.0.1:8080",
		generate,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/urls",
		strings.NewReader(`{"url":"https://example.com/new"}`),
	)
	defer request.Body.Close()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code, http.StatusCreated, recorder.Body.String(),
		)
	}

	if calls != 2 {
		t.Errorf("generator calls = %d, want 2", calls)
	}

	var response struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != "fresh456" {
		t.Errorf("response code = %q, want fresh456", response.Code)
	}

	gotOriginal, exists := store.get(original.Code)
	if !exists || gotOriginal != original {
		t.Errorf(
			"original record = (%+v, %t), want (%+v, true)",
			gotOriginal, exists, original,
		)
	}

	wantNew := urlRecord{
		Code:        "fresh456",
		OriginalURL: "https://example.com/new",
	}
	gotNew, exists := store.get(wantNew.Code)
	if !exists || gotNew != wantNew {
		t.Errorf(
			"new record = (%+v, %t), want (%+v, true)",
			gotNew, exists, wantNew,
		)
	}
}

func TestCreateURLHandlerCollisionLimit(t *testing.T) {
	store := newMemoryStore()
	original := urlRecord{
		Code:        "taken123",
		OriginalURL: "https://example.com/original",
	}

	if err := store.save(original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	calls := 0
	generate := func() (string, error) {
		calls++
		return original.Code, nil
	}

	handler := newCreateURLHandler(
		store,
		"http://127.0.0.1:8080",
		generate,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/urls",
		strings.NewReader(`{"url":"https://example.com/new"}`),
	)
	defer request.Body.Close()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf(
			"status = %d, want %d; body = %s",
			recorder.Code, http.StatusServiceUnavailable, recorder.Body.String(),
		)
	}

	if calls != 5 {
		t.Errorf("generator calls = %d, want 5", calls)
	}

	got, exists := store.get(original.Code)
	if !exists || got != original {
		t.Errorf(
			"original record = (%+v, %t), want (%+v, true)",
			got, exists, original,
		)
	}

	store.mu.RLock()
	count := len(store.urls)
	store.mu.RUnlock()

	if count != 1 {
		t.Errorf("stored records = %d, want 1", count)
	}
}
