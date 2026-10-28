package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAPIKey(t *testing.T) {
	const key = "test-management-key"

	tests := []struct {
		name       string
		header     string
		authorized bool
	}{
		{name: "missing credentials"},
		{
			name:   "wrong key",
			header: "Bearer incorrect-key",
		},
		{
			name:   "wrong scheme",
			header: "Basic " + key,
		},
		{
			name:   "missing token",
			header: "Bearer",
		},
		{
			name:   "empty token",
			header: "Bearer ",
		},
		{
			name:   "extra token content",
			header: "Bearer " + key + " extra",
		},
		{
			name:       "valid credentials",
			header:     "Bearer " + key,
			authorized: true,
		},
		{
			name:       "case insensitive scheme",
			header:     "bearer " + key,
			authorized: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})

			handler, err := requireAPIKey(key, next)
			if err != nil {
				t.Fatalf("configure middleware: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodDelete,
				"/api/v1/urls/go-guide",
				nil,
			)
			if tt.header != "" {
				request.Header.Set("Authorization", tt.header)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			wantStatus := http.StatusUnauthorized
			wantChallenge := "Bearer"
			if tt.authorized {
				wantStatus = http.StatusNoContent
				wantChallenge = ""
			}

			if recorder.Code != wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, wantStatus)
			}

			if called != tt.authorized {
				t.Errorf("protected handler called = %t, want %t", called, tt.authorized)
			}

			if got := recorder.Header().Get("WWW-Authenticate"); got != wantChallenge {
				t.Errorf("WWW-Authenticate = %q, want %q", got, wantChallenge)
			}
		})
	}
}

func TestRequireAPIKeyRejectsEmptyConfiguration(t *testing.T) {
	for _, key := range []string{"", " \t"} {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("protected handler must not run")
		})

		handler, err := requireAPIKey(key, next)
		if err == nil {
			t.Fatal("expected empty-key configuration error")
		}
		if handler != nil {
			t.Fatal("expected nil handler for invalid configuration")
		}
	}
}
