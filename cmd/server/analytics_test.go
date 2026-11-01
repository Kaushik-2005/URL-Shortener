package main

import (
	"testing"
	"time"
)

func TestAnalyticsDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "before UTC midnight",
			input: "2026-10-30T23:59:59.999999999Z",
			want:  "2026-10-30",
		},
		{
			name:  "exactly UTC midnight",
			input: "2026-10-31T00:00:00Z",
			want:  "2026-10-31",
		},
		{
			name:  "positive offset gives previous UTC date",
			input: "2026-10-31T00:30:00+05:30",
			want:  "2026-10-30",
		},
		{
			name:  "negative offset gives next UTC date",
			input: "2026-10-30T23:30:00-02:00",
			want:  "2026-10-31",
		},
		{
			name:  "equivalent instant in UTC",
			input: "2026-10-30T19:00:00Z",
			want:  "2026-10-30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at, err := time.Parse(time.RFC3339Nano, tt.input)
			if err != nil {
				t.Fatalf("parse test timestamp: %v", err)
			}

			if got := analyticsDate(at); got != tt.want {
				t.Errorf("analyticsDate = %q, want %q", got, tt.want)
			}
		})
	}
}
