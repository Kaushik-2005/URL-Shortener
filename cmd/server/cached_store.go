package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const cacheOperationTimeout = 200 * time.Millisecond

type cachedStore struct {
	primary urlStore
	cache   urlCache
	maxTTL  time.Duration
	mu      sync.RWMutex
}

func (s *cachedStore) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := ctx.Err(); err != nil {
		return urlRecord{}, err
	}

	readCtx, cancelRead := context.WithTimeout(ctx, cacheOperationTimeout)
	record, err := s.cache.get(readCtx, code)
	cancelRead()

	if err == nil {
		return record, nil
	}

	if err := ctx.Err(); err != nil {
		return urlRecord{}, err
	}

	if !errors.Is(err, errCacheMiss) {
		log.Printf("Cache lookup failed: %v", err)
	}

	record, err = s.primary.get(ctx, code)
	if err != nil {
		return urlRecord{}, err
	}

	ttl := calculateCacheTTL(record, s.maxTTL, time.Now())
	if ttl <= 0 {
		return record, nil
	}

	writeCtx, cancelWrite := context.WithTimeout(ctx, cacheOperationTimeout)
	cacheErr := s.cache.set(writeCtx, record, ttl)
	cancelWrite()

	if cacheErr != nil {
		log.Printf("Cache population failed: %v", cacheErr)
	}

	return record, nil
}

func (s *cachedStore) save(
	ctx context.Context,
	record urlRecord,
) error {
	return s.primary.save(ctx, record)
}

func (s *cachedStore) delete(
	ctx context.Context,
	code string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	invalidateCtx, cancel := context.WithTimeout(
		ctx,
		cacheOperationTimeout,
	)
	err := s.cache.delete(invalidateCtx, code)
	cancel()

	if err != nil {
		return fmt.Errorf("invalidate URL before deletion: %w", err)
	}

	return s.primary.delete(ctx, code)
}

var _ urlStore = (*cachedStore)(nil)
