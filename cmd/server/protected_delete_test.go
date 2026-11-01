package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedDelete(t *testing.T) {
	const key = "test-management-key"

	tests := []struct {
		name       string
		header     string
		wantStatus int
		deleted    bool
	}{
		{
			name:       "missing credentials",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong credentials",
			header:     "Bearer incorrect-key",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid credentials",
			header:     "Bearer " + key,
			wantStatus: http.StatusNoContent,
			deleted:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			record := urlRecord{
				Code:        "go-guide",
				OriginalURL: "https://example.com/guide",
			}

			if err := store.save(context.Background(), record); err != nil {
				t.Fatalf("save record: %v", err)
			}

			before, err := store.get(context.Background(), record.Code)
			if err != nil {
				t.Fatalf("get record before deletion request: %v", err)
			}
			if before.ID <= 0 {
				t.Fatalf("stored ID = %d, want positive ID", before.ID)
			}

			handler, err := requireAPIKey(key, newDeleteURLHandler(store))
			if err != nil {
				t.Fatalf("configure protected handler: %v", err)
			}

			request := httptest.NewRequest(
				http.MethodDelete,
				"/api/v1/urls/"+record.Code,
				nil,
			)
			request.SetPathValue("code", record.Code)
			if tt.header != "" {
				request.Header.Set("Authorization", tt.header)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}

			saved, lookupErr := store.get(context.Background(), record.Code)

			if tt.deleted {
				if !errors.Is(lookupErr, errURLNotFound) {
					t.Errorf("lookup error = %v, want errURLNotFound", lookupErr)
				}
				if saved != (urlRecord{}) {
					t.Errorf("deleted record = %+v, want empty record", saved)
				}
				return
			}

			if lookupErr != nil {
				t.Fatalf("unauthorized request removed the record: %v", lookupErr)
			}
			if saved != before {
				t.Errorf("record = %+v, want unchanged %+v", saved, before)
			}
		})
	}
}
