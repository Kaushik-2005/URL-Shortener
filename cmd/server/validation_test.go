package main

import "testing"

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "http URL",
			input: "http://example.com",
		},
		{
			name:  "https URL with path and query",
			input: "https://example.com/articles/go?page=2",
		},
		{
			name:  "URL with port",
			input: "http://localhost:8080/healthz",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "relative path",
			input:   "/articles/go",
			wantErr: true,
		},
		{
			name:    "unsupported scheme",
			input:   "ftp://example.com",
			wantErr: true,
		},
		{
			name:    "missing hostname",
			input:   "https:///articles/go",
			wantErr: true,
		},
		{
			name:    "invalid escape",
			input:   "https://example.com/%zz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"validateURL(%q) error = %v, wantErr%t",
					tt.input, err, tt.wantErr,
				)
			}
		})
	}
}
