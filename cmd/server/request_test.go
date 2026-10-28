package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecodeCreateRequest(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantURL      string
		wantErr      bool
		wantTooLarge bool
	}{
		{
			name:    "valid request",
			body:    `{"url":"https://example.com"}`,
			wantURL: "https://example.com",
		},
		{
			name:    "trailing whitespace",
			body:    "{\"url\":\"https://example.com\"}\n\t ",
			wantURL: "https://example.com",
		},
		{
			name:    "unknown field",
			body:    `{"url":"https://example.com","admin":true}`,
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			body:    `{"url":`,
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    "",
			wantErr: true,
		},
		{
			name:    "wrong field type",
			body:    `{"url":123}`,
			wantErr: true,
		},
		{
			name:    "second JSON value",
			body:    `{"url":"https://example.com"} {}`,
			wantErr: true,
		},
		{
			name:    "trailing garbage",
			body:    `{"url":"https://example.com"} garbage`,
			wantErr: true,
		},
		{
			name:         "oversized body",
			body:         `{"url":"` + strings.Repeat("x", 4096) + `"}`,
			wantErr:      true,
			wantTooLarge: true,
		},
		{
			name:         "oversized trailing whitespace",
			body:         `{"url":"https://example.com"}` + strings.Repeat(" ", 4096),
			wantErr:      true,
			wantTooLarge: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/urls",
				strings.NewReader(tt.body),
			)
			defer request.Body.Close()

			recorder := httptest.NewRecorder()
			got, err := decodeCreateRequest(recorder, request)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}

			var sizeErr *http.MaxBytesError
			if errors.As(err, &sizeErr) != tt.wantTooLarge {
				t.Fatalf(
					"error = %v, wantTooLarge %t",
					err, tt.wantTooLarge,
				)
			}

			if err == nil && got.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tt.wantURL)
			}
		})
	}
}

func TestDecodeCreateRequestAlias(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantAlias    string
		wantAliasSet bool
		wantErr      bool
	}{
		{
			name: "omitted alias",
			body: `{"url":"https://example.com"}`,
		},
		{
			name: "null alias",
			body: `{"url":"https://example.com","alias":null}`,
		},
		{
			name:         "empty alias",
			body:         `{"url":"https://example.com","alias":""}`,
			wantAliasSet: true,
		},
		{
			name:         "custom alias",
			body:         `{"url":"https://example.com","alias":"Go-guide"}`,
			wantAlias:    "Go-guide",
			wantAliasSet: true,
		},
		{
			name:    "wrong alias type",
			body:    `{"url":"https://example.com","alias":123}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/urls",
				strings.NewReader(tt.body),
			)
			defer request.Body.Close()

			recorder := httptest.NewRecorder()
			got, err := decodeCreateRequest(recorder, request)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if !tt.wantAliasSet {
				if got.Alias != nil {
					t.Fatalf("alias = %q, want nil", *got.Alias)
				}
				return
			}

			if got.Alias == nil {
				t.Fatal("alias is nil, want a supplied string")
			}

			if *got.Alias != tt.wantAlias {
				t.Errorf("alias = %q, want %q", *got.Alias, tt.wantAlias)
			}
		})
	}
}

func TestDecodeCreateRequestExpiry(t *testing.T) {
	future := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	past := time.Date(2020, time.January, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		body    string
		want    *time.Time
		wantErr bool
	}{
		{
			name: "omitted expiry",
			body: `{"url":"https://example.com"}`,
		},
		{
			name: "null expiry",
			body: `{"url":"https://example.com","expires_at":null}`,
		},
		{
			name: "UTC timestamp",
			body: `{"url":"https://example.com","expires_at":"2030-01-01T12:00:00Z"}`,
			want: &future,
		},
		{
			name: "equivalent offset timestamp",
			body: `{"url":"https://example.com","expires_at":"2030-01-01T17:30:00+05:30"}`,
			want: &future,
		},
		{
			name: "past timestamp still decodes",
			body: `{"url":"https://example.com","expires_at":"2020-01-01T12:00:00Z"}`,
			want: &past,
		},
		{
			name:    "missing timezone",
			body:    `{"url":"https://example.com","expires_at":"2030-01-01T12:00:00"}`,
			wantErr: true,
		},
		{
			name:    "wrong expiry type",
			body:    `{"url":"https://example.com","expires_at":123}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/urls",
				strings.NewReader(tt.body),
			)
			defer request.Body.Close()

			recorder := httptest.NewRecorder()
			got, err := decodeCreateRequest(recorder, request)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if tt.want == nil {
				if got.ExpiresAt != nil {
					t.Errorf("expiry = %v, want nil", got.ExpiresAt)
				}
				return
			}

			if got.ExpiresAt == nil {
				t.Fatal("expiry is nil, want decoded timestamp")
			}

			if !got.ExpiresAt.Equal(*tt.want) {
				t.Errorf("expiry = %v, want instant %v", *got.ExpiresAt, *tt.want)
			}
		})
	}
}
