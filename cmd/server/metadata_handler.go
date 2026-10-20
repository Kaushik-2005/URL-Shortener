package main

import (
	"net/http"
)

func newMetadataHandler(store urlRepository) http.HandlerFunc {
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

		writeJSON(w, http.StatusOK, response)
	}
}
