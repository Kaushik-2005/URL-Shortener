package main

import (
	"log/slog"
	"net/http"
	"time"
)

func withRequestLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		logger.InfoContext(
			r.Context(),
			"HTTP request completed",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"route", r.Pattern,
			"duration_ms", float64(time.Since(start))/float64(time.Millisecond),
		)
	})
}
