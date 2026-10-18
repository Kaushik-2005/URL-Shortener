package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
