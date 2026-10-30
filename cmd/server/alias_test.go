package main

import (
	"strings"
	"testing"
)

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{
			name:  "minimum length",
			alias: "abc",
		},
		{
			name:  "maximum length",
			alias: strings.Repeat("a", 32),
		},
		{
			name:  "allowed characters",
			alias: "Go-guide_2026",
		},
		{
			name:    "empty alias",
			alias:   "",
			wantErr: true,
		},
		{
			name:    "too short",
			alias:   "ab",
			wantErr: true,
		},
		{
			name:    "too long",
			alias:   strings.Repeat("a", 33),
			wantErr: true,
		},
		{
			name:    "reserved health endpoint",
			alias:   "healthz",
			wantErr: true,
		},
		{
			name:    "reserved name with mixed case",
			alias:   "HeAlThZ",
			wantErr: true,
		},
		{
			name:    "reserved API prefix",
			alias:   "api",
			wantErr: true,
		},
		{
			name:    "space",
			alias:   "go guide",
			wantErr: true,
		},
		{
			name:    "slash",
			alias:   "go/guide",
			wantErr: true,
		},
		{
			name:    "query delimiter",
			alias:   "go?guide",
			wantErr: true,
		},
		{
			name:    "Unicode letters",
			alias:   "café",
			wantErr: true,
		},
		{
			name:    "reserved readniess endpoint",
			alias:   "readyz",
			wantErr: true,
		},
		{
			name:    "reserved readiness endpoint with mixed case",
			alias:   "ReAdYz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAlias(tt.alias)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"validateAlias(%q) error = %v, wantErr %t",
					tt.alias, err, tt.wantErr,
				)
			}
		})
	}
}
