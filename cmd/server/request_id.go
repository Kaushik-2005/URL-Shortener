package main

import (
	"context"
	"crypto/rand"
	"net/http"
)

type requestIDContextKey struct{}

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := rand.Text()

		w.Header().Set("X-Request-ID", id)

		ctx := context.WithValue(
			r.Context(),
			requestIDContextKey{},
			id,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
