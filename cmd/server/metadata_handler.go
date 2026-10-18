package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func newMetadataHandler(store *memoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		record, exists := store.get(code)
		if !exists {
			http.NotFound(w, r)
			return
		}

		response := struct {
			Code string `json:"code"`
			URL  string `json:"url"`
		}{
			Code: record.Code,
			URL:  record.OriginalURL,
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Write metadata response: %v", err)
		}
	}
}
