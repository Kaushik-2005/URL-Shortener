package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeoutPreservesParent(t *testing.T) {
	type contextKey struct{}

	deadline := time.Now().Add(time.Minute)
	parent, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	parent = context.WithValue(parent, contextKey{}, "preserved")

	var child context.Context

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		child = r.Context()

		gotDeadline, ok := child.Deadline()
		if !ok {
			t.Error("child context has no deadline")
		} else if !gotDeadline.Equal(deadline) {
			t.Errorf("deadline = %v, want %v", gotDeadline, deadline)
		}

		if got := child.Value(contextKey{}); got != "preserved" {
			t.Errorf("context value = %v, want preserved", got)
		}

		if err := child.Err(); err != nil {
			t.Errorf("child canceled before handler finished: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(parent)
	recorder := httptest.NewRecorder()

	withRequestTimeout(5*time.Minute, next).ServeHTTP(recorder, request)

	if child == nil {
		t.Fatal("downstream handler was not called")
	}
	if !errors.Is(child.Err(), context.Canceled) {
		t.Errorf("child error after return = %v, want context.Canceled", child.Err())
	}
	if err := parent.Err(); err != nil {
		t.Errorf("middleware canceled parent: %v", err)
	}
	if recorder.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", recorder.Code)
	}
}

func TestRequestTimeoutCancellation(t *testing.T) {
	tests := []struct {
		name         string
		timeout      time.Duration
		cancelParent bool
		wantErr      error
	}{
		{
			name:    "timeout already expired",
			timeout: 0,
			wantErr: context.DeadlineExceeded,
		},
		{
			name:         "parent already canceled",
			timeout:      time.Minute,
			cancelParent: true,
			wantErr:      context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.cancelParent {
				cancel()
			}

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true

				if err := r.Context().Err(); !errors.Is(err, tt.wantErr) {
					t.Errorf("context error = %v, want %v", err, tt.wantErr)
				}
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request = request.WithContext(parent)
			recorder := httptest.NewRecorder()

			withRequestTimeout(tt.timeout, next).ServeHTTP(recorder, request)

			if !called {
				t.Error("downstream handler was not called")
			}
		})
	}
}
