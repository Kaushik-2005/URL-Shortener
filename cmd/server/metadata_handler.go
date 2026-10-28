package main

import (
	"net/http"
	"time"
)

func newMetadataHandler(store urlRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		record, err := store.get(r.Context(), code)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		if record.isExpired(time.Now()) {
			writeServiceError(w, errURLExpired)
			return
		}

		response := struct {
			Code      string     `json:"code"`
			URL       string     `json:"url"`
			ExpiresAt *time.Time `json:"expires_at,omitempty"`
		}{
			Code:      record.Code,
			URL:       record.OriginalURL,
			ExpiresAt: record.ExpiresAt,
		}

		writeJSON(w, http.StatusOK, response)
	}
}
