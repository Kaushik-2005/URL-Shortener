package main

import (
	"errors"
	"time"
)

var errInvalidExpiry = errors.New("expiry must be in the future")
var errURLExpired = errors.New("URL has expired")

func (r urlRecord) isExpired(now time.Time) bool {
	if r.ExpiresAt == nil {
		return false
	}

	return !now.Before(*r.ExpiresAt)
}

func prepareExpiry(
	expiresAt *time.Time,
	now time.Time,
) (*time.Time, error) {
	if expiresAt == nil {
		return nil, nil
	}

	deadline := expiresAt.UTC().Truncate(time.Microsecond)
	if !deadline.After(now) {
		return nil, errInvalidExpiry
	}

	return &deadline, nil
}
