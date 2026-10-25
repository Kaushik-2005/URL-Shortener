package main

import "net/http"

func newRedirectHandler(store urlRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		record, err := store.get(r.Context(), code)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		http.Redirect(w, r, record.OriginalURL, http.StatusFound)
	}
}
