package main

import (
	"testing"
	"time"
)

func TestCalculateCacheTTL(t *testing.T) {
	now := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC)
	soon := now.Add(2 * time.Minute)
	later := now.Add(10 * time.Minute)
	past := now.Add(-time.Second)
	exact := now
	almost := now.Add(999 * time.Microsecond)
	fractional := now.Add(2*time.Second + 1234*time.Microsecond)

	tests := []struct {
		name      string
		expiresAt *time.Time
		maxTTL    time.Duration
		want      time.Duration
	}{
		{
			name:   "permanent link",
			maxTTL: 5 * time.Minute,
			want:   5 * time.Minute,
		},
		{
			name:      "expiry limits TTL",
			expiresAt: &soon,
			maxTTL:    5 * time.Minute,
			want:      2 * time.Minute,
		},
		{
			name:      "maximum limits TTL",
			expiresAt: &later,
			maxTTL:    5 * time.Minute,
			want:      5 * time.Minute,
		},
		{
			name:      "at expiry",
			expiresAt: &exact,
			maxTTL:    5 * time.Minute,
			want:      0,
		},
		{
			name:      "already expired",
			expiresAt: &past,
			maxTTL:    5 * time.Minute,
			want:      0,
		},
		{
			name:      "less than one millisecond remaining",
			expiresAt: &almost,
			maxTTL:    5 * time.Minute,
			want:      0,
		},
		{
			name:      "round down to milliseconds",
			expiresAt: &fractional,
			maxTTL:    5 * time.Minute,
			want:      2*time.Second + time.Millisecond,
		},
		{
			name:   "zero maximum",
			maxTTL: 0,
			want:   0,
		},
		{
			name:   "negative maximum",
			maxTTL: -time.Second,
			want:   0,
		},
		{
			name:   "sub-millisecond maximum",
			maxTTL: 500 * time.Microsecond,
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := urlRecord{ExpiresAt: tt.expiresAt}

			got := calculateCacheTTL(record, tt.maxTTL, now)
			if got != tt.want {
				t.Errorf("TTL = %v, want %v", got, tt.want)
			}
		})
	}
}
