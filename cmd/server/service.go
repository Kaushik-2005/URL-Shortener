package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	errInvalidURL              = errors.New("invalid URL")
	errCodeAllocationExhausted = errors.New("short-code allocation attempts exhausted")
	errInvalidAlias            = errors.New("invalid alias")
	errAliasTaken              = errors.New("alias already exists")
)

type urlService struct {
	repository urlRepository
	generate   func() (string, error)
}

func (s *urlService) create(
	ctx context.Context,
	rawURL string,
	expiresAt *time.Time,
) (urlRecord, error) {
	if err := validateURL(rawURL); err != nil {
		return urlRecord{}, fmt.Errorf("%w: %v", errInvalidURL, err)
	}

	deadline, err := prepareExpiry(expiresAt, time.Now())
	if err != nil {
		return urlRecord{}, err
	}

	record := urlRecord{
		OriginalURL: rawURL,
		ExpiresAt:   deadline,
	}

	for attempt := 0; attempt < 5; attempt++ {
		code, err := s.generate()
		if err != nil {
			return urlRecord{}, fmt.Errorf("generate code: %w", err)
		}

		record.Code = code

		err = s.repository.save(ctx, record)
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

func (s *urlService) createWithAlias(
	ctx context.Context,
	rawURL string,
	alias string,
	expiresAt *time.Time,
) (urlRecord, error) {
	if err := validateURL(rawURL); err != nil {
		return urlRecord{}, fmt.Errorf("%w: %v", errInvalidURL, err)
	}

	if err := validateAlias(alias); err != nil {
		return urlRecord{}, fmt.Errorf("%w: %v", errInvalidAlias, err)
	}

	deadline, err := prepareExpiry(expiresAt, time.Now())
	if err != nil {
		return urlRecord{}, err
	}

	record := urlRecord{
		OriginalURL: rawURL,
		Code:        alias,
		ExpiresAt:   deadline,
	}

	err = s.repository.save(ctx, record)
	if errors.Is(err, errCodeExists) {
		return urlRecord{}, errAliasTaken
	}
	if err != nil {
		return urlRecord{}, fmt.Errorf("save aliased URL: %w", err)
	}

	return record, nil
}
