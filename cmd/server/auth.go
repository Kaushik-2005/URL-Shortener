package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

func requireAPIKey(
	apiKey string,
	next http.Handler,
) (http.Handler, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("management API key must not be empty")
	}

	expectedHash := sha256.Sum256([]byte(apiKey))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(
			r.Header.Get("Authorization"),
			" ",
		)
		providedHash := sha256.Sum256([]byte(token))

		if !ok ||
			!strings.EqualFold(scheme, "Bearer") ||
			token == "" ||
			subtle.ConstantTimeCompare(
				providedHash[:],
				expectedHash[:],
			) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}), nil
}
