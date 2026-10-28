package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errCodeExists = errors.New("Short code already exists")

type urlRecord struct {
	Code        string
	OriginalURL string
	ExpiresAt   *time.Time
}

type memoryStore struct {
	mu   sync.RWMutex
	urls map[string]urlRecord
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		urls: make(map[string]urlRecord),
	}
}

func (s *memoryStore) save(ctx context.Context, record urlRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[record.Code]; exists {
		return errCodeExists
	}

	s.urls[record.Code] = record
	return nil
}

func (s *memoryStore) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := ctx.Err(); err != nil {
		return urlRecord{}, err
	}

	record, exists := s.urls[code]
	if !exists {
		return urlRecord{}, errURLNotFound
	}

	return record, nil
}

func (s *memoryStore) delete(
	ctx context.Context,
	code string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if _, exists := s.urls[code]; !exists {
		return errURLNotFound
	}

	delete(s.urls, code)
	return nil
}
