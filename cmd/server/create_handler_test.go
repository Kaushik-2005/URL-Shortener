package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

	record, lookupErr := store.get(context.Background(), response.Code)
	if lookupErr != nil {
		t.Fatalf("get created record: %v", lookupErr)
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
		{
			name:       "past expiry with random code",
			body:       `{"url":"https://example.com","expires_at":"2020-01-01T12:00:00Z"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "past expiry with custom alias",
			body:       `{"url":"https://example.com","alias":"go-guide","expires_at":"2020-01-01T12:00:00Z"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "expiry missing timezone",
			body:       `{"url":"https://example.com","expires_at":"2030-01-01T12:00:00"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong expiry type",
			body:       `{"url":"https://example.com","expires_at":123}`,
			wantStatus: http.StatusBadRequest,
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

	if err := store.save(context.Background(), original); err != nil {
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

	gotOriginal, lookupErr := store.get(context.Background(), original.Code)
	if lookupErr != nil {
		t.Fatalf("get original record: %v", lookupErr)
	}

	if gotOriginal != original {
		t.Errorf("original record = %+v, want %+v", gotOriginal, original)
	}

	wantNew := urlRecord{
		Code:        "fresh456",
		OriginalURL: "https://example.com/new",
	}
	gotNew, lookupErr := store.get(context.Background(), wantNew.Code)
	if lookupErr != nil {
		t.Fatalf("get new record: %v", lookupErr)
	}

	if gotNew != wantNew {
		t.Errorf("new record = %+v, want %+v", gotNew, wantNew)
	}
}

func TestCreateURLHandlerCollisionLimit(t *testing.T) {
	store := newMemoryStore()
	original := urlRecord{
		Code:        "taken123",
		OriginalURL: "https://example.com/original",
	}

	if err := store.save(context.Background(), original); err != nil {
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

	got, lookupErr := store.get(context.Background(), original.Code)
	if lookupErr != nil {
		t.Fatalf("get original after retry exhaustion: %v", lookupErr)
	}

	if got != original {
		t.Errorf("original record = %+v, want %+v", got, original)
	}

	store.mu.RLock()
	count := len(store.urls)
	store.mu.RUnlock()

	if count != 1 {
		t.Errorf("stored records = %d, want 1", count)
	}
}

func TestCreateURLHandlerCustomAliasSuccess(t *testing.T) {
	store := newMemoryStore()
	baseURL := "http://127.0.0.1:8080"
	generatorCalls := 0

	handler := newCreateURLHandler(
		store,
		baseURL,
		func() (string, error) {
			generatorCalls++
			return "random12", nil
		},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/urls",
		strings.NewReader(
			`{"url":"https://example.com/guide","alias":"Go-guide"}`,
		),
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
		t.Fatalf("expected response to end after one JSON value; got %v", err)
	}

	want := urlRecord{
		Code:        "Go-guide",
		OriginalURL: "https://example.com/guide",
	}

	if response.Code != want.Code || response.URL != want.OriginalURL {
		t.Errorf("response = %+v, want code and URL from %+v", response, want)
	}

	if response.ShortURL != baseURL+"/"+want.Code {
		t.Errorf("short URL = %q, want %q", response.ShortURL, baseURL+"/"+want.Code)
	}

	saved, err := store.get(context.Background(), want.Code)
	if err != nil {
		t.Fatalf("get saved alias: %v", err)
	}

	if saved != want {
		t.Errorf("saved record = %+v, want %+v", saved, want)
	}

	if generatorCalls != 0 {
		t.Errorf("generator calls = %d, want 0", generatorCalls)
	}
}

func TestCreateURLHandlerRejectsCustomAlias(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "empty alias",
			body:       `{"url":"https://example.com/new","alias":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "reserved alias",
			body:       `{"url":"https://example.com/new","alias":"HeAlThZ"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid character",
			body:       `{"url":"https://example.com/new","alias":"go/guide"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong alias type",
			body:       `{"url":"https://example.com/new","alias":123}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "occupied alias",
			body:       `{"url":"https://example.com/new","alias":"taken-guide"}`,
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			original := urlRecord{
				Code:        "taken-guide",
				OriginalURL: "https://example.com/original",
			}

			if err := store.save(context.Background(), original); err != nil {
				t.Fatalf("save original: %v", err)
			}

			generatorCalls := 0
			handler := newCreateURLHandler(
				store,
				"http://127.0.0.1:8080",
				func() (string, error) {
					generatorCalls++
					return "random12", nil
				},
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

			saved, err := store.get(context.Background(), original.Code)
			if err != nil {
				t.Fatalf("get original: %v", err)
			}

			if saved != original {
				t.Errorf("original record = %+v, want %+v", saved, original)
			}

			store.mu.RLock()
			count := len(store.urls)
			store.mu.RUnlock()

			if count != 1 {
				t.Errorf("stored records = %d, want 1", count)
			}

			if generatorCalls != 0 {
				t.Errorf("generator calls = %d, want 0", generatorCalls)
			}
		})
	}
}

func TestCreateURLHandlerExpiry(t *testing.T) {
	tests := []struct {
		name  string
		alias string
	}{
		{name: "random code"},
		{name: "custom alias", alias: "expiry-guide"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			handler := newCreateURLHandler(
				store,
				"http://127.0.0.1:8080",
				func() (string, error) {
					return "random12", nil
				},
			)

			expiry := time.Now().UTC().Truncate(time.Second).Add(
				time.Hour + 123456789*time.Nanosecond,
			).In(time.FixedZone("UTC+0530", 5*3600+30*60))
			wantExpiry := expiry.UTC().Truncate(time.Microsecond)

			payload := map[string]any{
				"url":        "https://example.com/expiry",
				"expires_at": expiry,
			}
			if tt.alias != "" {
				payload["alias"] = tt.alias
			}

			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("encode request: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/urls",
				strings.NewReader(string(body)),
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

			var response struct {
				Code      string     `json:"code"`
				ExpiresAt *time.Time `json:"expires_at"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if tt.alias != "" && response.Code != tt.alias {
				t.Errorf("code = %q, want %q", response.Code, tt.alias)
			}

			if response.ExpiresAt == nil {
				t.Fatal("response expiry is missing")
			}
			if !response.ExpiresAt.Equal(wantExpiry) {
				t.Errorf("response expiry = %v, want %v", *response.ExpiresAt, wantExpiry)
			}
			if response.ExpiresAt.Location() != time.UTC {
				t.Errorf("response expiry location = %v, want UTC", response.ExpiresAt.Location())
			}

			saved, err := store.get(context.Background(), response.Code)
			if err != nil {
				t.Fatalf("get saved record: %v", err)
			}
			if saved.ExpiresAt == nil {
				t.Fatal("stored expiry is missing")
			}
			if !saved.ExpiresAt.Equal(wantExpiry) {
				t.Errorf("stored expiry = %v, want %v", *saved.ExpiresAt, wantExpiry)
			}
		})
	}
}
