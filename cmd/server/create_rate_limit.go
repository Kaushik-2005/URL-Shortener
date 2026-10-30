package main

import (
	"net/http"
	"time"
)

func withCreateRateLimit(
	limiter *createLimiter,
	now func() time.Time,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(now()) {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(
				w,
				"creation rate limit exceeded",
				http.StatusTooManyRequests,
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}
