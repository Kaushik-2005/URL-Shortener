package main

import (
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
	default:
		log.Printf("Service error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
