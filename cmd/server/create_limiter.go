package main

import (
	"errors"
	"sync"
	"time"
)

type createLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	windowStart time.Time
	count       int
}

func newCreateLimiter(
	limit int,
	window time.Duration,
	now time.Time,
) (*createLimiter, error) {
	if limit <= 0 {
		return nil, errors.New("creation limit must be positive")
	}
	if window <= 0 {
		return nil, errors.New("creation window must be positive")
	}

	return &createLimiter{
		limit:       limit,
		window:      window,
		windowStart: now,
	}, nil
}

func (l *createLimiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.windowStart) >= l.window {
		l.windowStart = now
		l.count = 0
	}

	if l.count >= l.limit {
		return false
	}

	l.count++
	return true
}
