package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type createURLRequest struct {
	URL string `json:"url"`
}

func decodeCreateRequest(
	w http.ResponseWriter,
	r *http.Request,
) (createURLRequest, error) {
	const maxBodyBytes = 4 * 1024

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request createURLRequest
	if err := decoder.Decode(&request); err != nil {
		return createURLRequest{}, fmt.Errorf("decode request: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return createURLRequest{}, fmt.Errorf("read after request: %w", err)
		}

		return createURLRequest{}, errors.New(
			"body must contain exactly one JSON value",
		)
	}

	return request, nil
}
