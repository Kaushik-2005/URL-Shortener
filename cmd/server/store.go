package main

import (
	"errors"
	"sync"
)

var errCodeExists = errors.New("Short code already exists")

type urlRecord struct {
	Code        string
	OriginalURL string
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

func (s *memoryStore) save(record urlRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[record.Code]; exists {
		return errCodeExists
	}

	s.urls[record.Code] = record
	return nil
}

func (s *memoryStore) get(code string) (urlRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.urls[code]
	return record, exists
}
