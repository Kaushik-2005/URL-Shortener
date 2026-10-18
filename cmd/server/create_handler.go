package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

func newCreateURLHandler(
	store *memoryStore,
	baseURL string,
	generate func() (string, error),
) http.HandlerFunc {
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

		if err := validateURL(request.URL); err != nil {
			http.Error(
				w,
				"url must be an absolute http or https URL with a hostname",
				http.StatusBadRequest,
			)
			return
		}

		record := urlRecord{OriginalURL: request.URL}
		saved := false

		for attempt := 0; attempt < 5; attempt++ {
			code, err := generate()
			if err != nil {
				log.Printf("Generate code: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			record.Code = code
			err = store.save(record)
			if errors.Is(err, errCodeExists) {
				continue
			}
			if err != nil {
				log.Printf("Save URL: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			saved = true
			break
		}

		if !saved {
			http.Error(w, "could not allocate a short code", http.StatusServiceUnavailable)
			return
		}

		response := struct {
			Code     string `json:"code"`
			URL      string `json:"url"`
			ShortURL string `json:"short_url"`
		}{
			Code:     record.Code,
			URL:      record.OriginalURL,
			ShortURL: baseURL + "/" + record.Code,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Write create response: %v", err)
		}
	}
}
