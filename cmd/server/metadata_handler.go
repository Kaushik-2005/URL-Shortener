package main

import (
	"net/http"
)

func newMetadataHandler(store urlRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		record, err := store.get(r.Context(), code)
		if err != nil {
			writeServiceError(w, err)
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
