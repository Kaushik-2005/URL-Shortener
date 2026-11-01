package main

import (
	"net/http"
	"time"
)

func newRedirectHandler(
	store urlRepository,
	recorder clickRecorder,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		record, err := store.get(r.Context(), code)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		at := time.Now()
		if record.isExpired(at) {
			writeServiceError(w, errURLExpired)
			return
		}

		if r.Method == http.MethodGet {
			if err := recorder.recordClick(r.Context(), record.ID, at); err != nil {
				writeServiceError(w, err)
				return
			}
		}

		http.Redirect(w, r, record.OriginalURL, http.StatusFound)
	}
}
