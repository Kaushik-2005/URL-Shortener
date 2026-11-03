package main

import (
	"io"
	"log/slog"
	"testing"
)

func TestRunRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		backend  string
		redisURL string
		wantErr  string
		baseURL  string
	}{
		{
			name:    "unknown storage backend",
			backend: "unsupported",
			wantErr: `unknown STORAGE_BACKEND: "unsupported"`,
		},
		{
			name:     "Redis with memory storage",
			backend:  "memory",
			redisURL: "redis://127.0.0.1:6379/0",
			wantErr:  "REDIS_URL requires STORAGE_BACKEND=postgres",
		},
		{
			name:    "public base URL with path prefix",
			backend: "memory",
			baseURL: "https://sho.rt/short",
			wantErr: "configure public base URL: public base URL must not include a path prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STORAGE_BACKEND", tt.backend)
			t.Setenv("REDIS_URL", tt.redisURL)
			t.Setenv("PUBLIC_BASE_URL", tt.baseURL)

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

			err := run(logger)
			if err == nil {
				t.Fatal("run returned nil, want configuration error")
			}

			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
