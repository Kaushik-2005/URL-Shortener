package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errCodeExists = errors.New("Short code already exists")

type urlRecord struct {
	ID          int64
	Code        string
	OriginalURL string
	ExpiresAt   *time.Time
}

type memoryStore struct {
	mu     sync.RWMutex
	urls   map[string]urlRecord
	nextID int64
	clicks map[int64]map[string]int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		urls:   make(map[string]urlRecord),
		clicks: make(map[int64]map[string]int64),
	}
}

func (s *memoryStore) save(ctx context.Context, record urlRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if _, exists := s.urls[record.Code]; exists {
		return errCodeExists
	}

	s.nextID++
	record.ID = s.nextID

	s.urls[record.Code] = record
	s.clicks[record.ID] = make(map[string]int64)
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

	record, exists := s.urls[code]
	if !exists {
		return errURLNotFound
	}

	delete(s.urls, code)
	delete(s.clicks, record.ID)
	return nil
}
