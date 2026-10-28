package main

import "net/http"

func newDeleteURLHandler(store urlDeleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		if err := store.delete(r.Context(), code); err != nil {
			writeServiceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
