package main

import (
	"errors"
	"fmt"
)

var (
	errInvalidURL              = errors.New("invalid URL")
	errCodeAllocationExhausted = errors.New(
		"short-code allocation attempts exhausted",
	)
)

type urlService struct {
	repository urlRepository
	generate   func() (string, error)
}

func (s *urlService) create(rawURL string) (urlRecord, error) {
	if err := validateURL(rawURL); err != nil {
		return urlRecord{}, fmt.Errorf("%w: %v", errInvalidURL, err)
	}

	record := urlRecord{OriginalURL: rawURL}

	for attempt := 0; attempt < 5; attempt++ {
		code, err := s.generate()
		if err != nil {
			return urlRecord{}, fmt.Errorf("generate code: %w", err)
		}

		record.Code = code

		err = s.repository.save(record)
		if errors.Is(err, errCodeExists) {
			continue
		}
		if err != nil {
			return urlRecord{}, fmt.Errorf("save URL: %w", err)
		}

		return record, nil
	}

	return urlRecord{}, errCodeAllocationExhausted
}
