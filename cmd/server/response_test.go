package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteServiceErrorDeadlineExceeded(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "direct error",
			err:  context.DeadlineExceeded,
		},
		{
			name: "wrapped error",
			err:  fmt.Errorf("private storage details: %w", context.DeadlineExceeded),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			writeServiceError(recorder, tt.err)

			if recorder.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", recorder.Code)
			}
			if got := recorder.Body.String(); got != "request deadline exceeded\n" {
				t.Errorf("body = %q, want safe deadline message", got)
			}
		})
	}
}
