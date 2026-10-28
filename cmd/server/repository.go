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

type urlDeleter interface {
	delete(ctx context.Context, code string) error
}

type urlStore interface {
	urlRepository
	urlDeleter
}

var _ urlStore = (*memoryStore)(nil)
var _ urlStore = (*postgresStore)(nil)
