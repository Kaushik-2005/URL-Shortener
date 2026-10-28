package main

import (
	"net/http"
	"time"
)

func newRedirectHandler(store urlRepository) http.HandlerFunc {
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

		http.Redirect(w, r, record.OriginalURL, http.StatusFound)
	}
}
