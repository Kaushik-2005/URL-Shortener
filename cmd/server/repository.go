package main

import (
	"context"
	"errors"
)

var errURLNotFound = errors.New("URL not found")

type urlRepository interface {
	save(ctx context.Context, record urlRecord) error
	get(ctx context.Context, code string) (urlRecord, error)
}

var _ urlRepository = (*memoryStore)(nil)
var _ urlRepository = (*postgresStore)(nil)
