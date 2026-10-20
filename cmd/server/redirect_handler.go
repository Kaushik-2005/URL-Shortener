package main

import "net/http"

func newRedirectHandler(store urlRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		record, exists := store.get(code)
		if !exists {
			http.NotFound(w, r)
			return
		}

		http.Redirect(w, r, record.OriginalURL, http.StatusFound)
	}
}
