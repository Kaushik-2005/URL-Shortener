package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("Write JSON response: %v", err)
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInvalidURL):
		http.Error(
			w,
			"url must be an absolute http or https URL with a hostname",
			http.StatusBadRequest,
		)
	case errors.Is(err, errCodeAllocationExhausted):
		http.Error(
			w,
			"could not allocate a short code",
			http.StatusServiceUnavailable,
		)
	case errors.Is(err, errURLNotFound):
		http.Error(w, "404 page not found", http.StatusNotFound)
	case errors.Is(err, errInvalidAlias):
		http.Error(
			w,
			"alias must be 3-32 ASCII letters, digits, hyphens, or underscores and must not be reserved",
			http.StatusBadRequest,
		)
	case errors.Is(err, errAliasTaken):
		http.Error(w, "alias already exists", http.StatusConflict)
	case errors.Is(err, errInvalidExpiry):
		http.Error(w, "expiry must be in the future", http.StatusBadRequest)
	case errors.Is(err, errURLExpired):
		http.Error(w, "URL has expired", http.StatusGone)
	case errors.Is(err, context.DeadlineExceeded):
		http.Error(
			w,
			"request deadline exceeded",
			http.StatusServiceUnavailable,
		)
	default:
		log.Printf("Service error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
