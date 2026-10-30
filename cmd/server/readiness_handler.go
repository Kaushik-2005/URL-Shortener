package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func newReadinessHandler(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()

		w.Header().Set("Cache-Control", "no-store")

		if err := check(ctx); err != nil {
			http.Error(
				w,
				"not ready",
				http.StatusServiceUnavailable,
			)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "OK")
	}
}
