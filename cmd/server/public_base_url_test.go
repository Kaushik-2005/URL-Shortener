package main

import "testing"

func TestNormalizePublicBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "HTTPS domain",
			input: "https://sho.rt",
			want:  "https://sho.rt",
		},
		{
			name:  "remove root slash",
			input: "https://sho.rt/",
			want:  "https://sho.rt",
		},
		{
			name:  "local address with port",
			input: "http://127.0.0.1:8081/",
			want:  "http://127.0.0.1:8081",
		},
		{
			name:  "IPv6 address",
			input: "http://[::1]:8080/",
			want:  "http://[::1]:8080",
		},
		{
			name:    "empty input",
			wantErr: true,
		},
		{
			name:    "relative URL",
			input:   "/short",
			wantErr: true,
		},
		{
			name:    "unsupported scheme",
			input:   "ftp://sho.rt",
			wantErr: true,
		},
		{
			name:    "missing hostname",
			input:   "https:///short",
			wantErr: true,
		},
		{
			name:    "credentials",
			input:   "https://user:password@sho.rt",
			wantErr: true,
		},
		{
			name:    "query",
			input:   "https://sho.rt?source=test",
			wantErr: true,
		},
		{
			name:    "empty query",
			input:   "https://sho.rt?",
			wantErr: true,
		},
		{
			name:    "fragment",
			input:   "https://sho.rt#section",
			wantErr: true,
		},
		{
			name:    "empty fragment",
			input:   "https://sho.rt#",
			wantErr: true,
		},
		{
			name:    "path prefix",
			input:   "https://sho.rt/short",
			wantErr: true,
		},
		{
			name:    "invalid escape",
			input:   "https://sho.rt/%zz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePublicBaseURL(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected configuration error")
				}
				if got != "" {
					t.Errorf("result = %q, want empty string", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("normalize public base URL: %v", err)
			}
			if got != tt.want {
				t.Errorf("result = %q, want %q", got, tt.want)
			}
		})
	}
}
