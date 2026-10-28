package main

import (
	"errors"
	"net/http"
	"time"
)

func newCreateURLHandler(
	store urlRepository,
	baseURL string,
	generate func() (string, error),
) http.HandlerFunc {
	service := &urlService{
		repository: store,
		generate:   generate,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		request, err := decodeCreateRequest(w, r)
		if err != nil {
			var sizeErr *http.MaxBytesError
			if errors.As(err, &sizeErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		var record urlRecord

		if request.Alias == nil {
			record, err = service.create(
				r.Context(),
				request.URL,
				request.ExpiresAt,
			)
		} else {
			record, err = service.createWithAlias(
				r.Context(),
				request.URL,
				*request.Alias,
				request.ExpiresAt,
			)
		}

		if err != nil {
			writeServiceError(w, err)
			return
		}

		response := struct {
			Code      string     `json:"code"`
			URL       string     `json:"url"`
			ShortURL  string     `json:"short_url"`
			ExpiresAt *time.Time `json:"expires_at,omitempty"`
		}{
			Code:      record.Code,
			URL:       record.OriginalURL,
			ShortURL:  baseURL + "/" + record.Code,
			ExpiresAt: record.ExpiresAt,
		}

		writeJSON(w, http.StatusCreated, response)
	}
}
