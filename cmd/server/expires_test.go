package main

import (
	"errors"
	"testing"
	"time"
)

func TestURLRecordIsExpired(t *testing.T) {
	deadline := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt *time.Time
		now       time.Time
		want      bool
	}{
		{
			name: "no expiry",
			now:  deadline.Add(time.Hour),
			want: false,
		},
		{
			name:      "before deadline",
			expiresAt: &deadline,
			now:       deadline.Add(-time.Nanosecond),
			want:      false,
		},
		{
			name:      "at deadline",
			expiresAt: &deadline,
			now:       deadline,
			want:      true,
		},
		{
			name:      "after deadline",
			expiresAt: &deadline,
			now:       deadline.Add(time.Nanosecond),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := urlRecord{
				Code:        "go-guide",
				OriginalURL: "https://example.com",
				ExpiresAt:   tt.expiresAt,
			}

			if got := record.isExpired(tt.now); got != tt.want {
				t.Errorf("isExpired(%v) = %t, want %t", tt.now, got, tt.want)
			}
		})
	}
}

func TestPrepareExpiry(t *testing.T) {
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)

	future := now.Add(time.Hour + 123456789*time.Nanosecond).In(
		time.FixedZone("UTC+0530", 5*3600+30*60),
	)
	wantFuture := future.UTC().Truncate(time.Microsecond)
	past := now.Add(-time.Second)
	exact := now
	tooClose := now.Add(time.Nanosecond)

	tests := []struct {
		name    string
		input   *time.Time
		want    *time.Time
		wantErr bool
	}{
		{
			name: "no expiry",
		},
		{
			name:  "normalize future expiry",
			input: &future,
			want:  &wantFuture,
		},
		{
			name:    "past expiry",
			input:   &past,
			wantErr: true,
		},
		{
			name:    "exactly now",
			input:   &exact,
			wantErr: true,
		},
		{
			name:    "future before truncation only",
			input:   &tooClose,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var original time.Time
			if tt.input != nil {
				original = *tt.input
			}

			got, err := prepareExpiry(tt.input, now)

			if tt.input != nil && *tt.input != original {
				t.Error("input timestamp was modified")
			}

			if tt.wantErr {
				if !errors.Is(err, errInvalidExpiry) {
					t.Fatalf("error = %v, want errInvalidExpiry", err)
				}
				if got != nil {
					t.Errorf("expiry = %v, want nil", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("prepare expiry: %v", err)
			}

			if tt.want == nil {
				if got != nil {
					t.Errorf("expiry = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expiry is nil, want normalized timestamp")
			}

			if !got.Equal(*tt.want) {
				t.Errorf("expiry = %v, want %v", *got, *tt.want)
			}

			if got.Location() != time.UTC {
				t.Errorf("location = %v, want UTC", got.Location())
			}

			if got == tt.input {
				t.Error("returned pointer aliases the input timestamp")
			}
		})
	}
}
