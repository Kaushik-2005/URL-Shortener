package main

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

type stubPrimaryStore struct {
	record     urlRecord
	getErr     error
	getCalls   int
	saveErr    error
	saveCalls  int
	saveRecord urlRecord
	deleteFunc func(context.Context, string) error
}

func (s *stubPrimaryStore) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	s.getCalls++

	if err := ctx.Err(); err != nil {
		return urlRecord{}, err
	}

	return s.record, s.getErr
}

func (s *stubPrimaryStore) save(
	ctx context.Context,
	record urlRecord,
) error {
	s.saveCalls++
	s.saveRecord = record

	if err := ctx.Err(); err != nil {
		return err
	}

	return s.saveErr
}

func (s *stubPrimaryStore) delete(
	ctx context.Context,
	code string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.deleteFunc == nil {
		return errors.New("unexpected primary delete")
	}
	return s.deleteFunc(ctx, code)
}

type stubCache struct {
	record     urlRecord
	getErr     error
	getCalls   int
	setErr     error
	setCalls   int
	setRecord  urlRecord
	setTTL     time.Duration
	deleteFunc func(context.Context, string) error
}

func (c *stubCache) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	c.getCalls++
	return c.record, c.getErr
}

func (c *stubCache) set(
	ctx context.Context,
	record urlRecord,
	ttl time.Duration,
) error {
	c.setCalls++
	c.setRecord = record
	c.setTTL = ttl
	return c.setErr
}

func (c *stubCache) delete(
	ctx context.Context,
	code string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.deleteFunc == nil {
		return errors.New("unexpected cache delete")
	}
	return c.deleteFunc(ctx, code)
}

func TestCachedStoreGetHit(t *testing.T) {
	want := urlRecord{
		ID:          41,
		Code:        "go-guide",
		OriginalURL: "https://example.com/guide",
	}

	primary := &stubPrimaryStore{
		getErr: errors.New("primary should not be queried"),
	}
	cache := &stubCache{record: want}
	store := &cachedStore{
		primary: primary,
		cache:   cache,
		maxTTL:  time.Minute,
	}

	got, err := store.get(context.Background(), want.Code)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != want {
		t.Errorf("record = %+v, want %+v", got, want)
	}

	if primary.getCalls != 0 {
		t.Errorf("primary lookups = %d, want 0", primary.getCalls)
	}
	if cache.getCalls != 1 {
		t.Errorf("cache lookups = %d, want 1", cache.getCalls)
	}
	if cache.setCalls != 0 {
		t.Errorf("cache writes = %d, want 0", cache.setCalls)
	}
}

func TestCachedStoreGetFallback(t *testing.T) {
	want := urlRecord{
		ID:          42,
		Code:        "go-guide",
		OriginalURL: "https://example.com/guide",
	}

	tests := []struct {
		name        string
		cacheGetErr error
		cacheSetErr error
	}{
		{
			name:        "cache miss",
			cacheGetErr: errCacheMiss,
		},
		{
			name:        "cache read failure",
			cacheGetErr: errors.New("Redis unavailable"),
		},
		{
			name:        "invalid cached data",
			cacheGetErr: errors.New("decode cached URL"),
		},
		{
			name:        "cache population failure",
			cacheGetErr: errCacheMiss,
			cacheSetErr: errors.New("Redis write failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &stubPrimaryStore{record: want}
			cache := &stubCache{
				getErr: tt.cacheGetErr,
				setErr: tt.cacheSetErr,
			}
			store := &cachedStore{
				primary: primary,
				cache:   cache,
				maxTTL:  time.Minute,
			}

			got, err := store.get(context.Background(), want.Code)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got != want {
				t.Errorf("record = %+v, want %+v", got, want)
			}

			if cache.getCalls != 1 {
				t.Errorf("cache lookups = %d, want 1", cache.getCalls)
			}
			if primary.getCalls != 1 {
				t.Errorf("primary lookups = %d, want 1", primary.getCalls)
			}
			if cache.setCalls != 1 {
				t.Errorf("cache writes = %d, want 1", cache.setCalls)
			}
			if cache.setRecord != want {
				t.Errorf("cache write record = %+v, want %+v", cache.setRecord, want)
			}
			if cache.setTTL != time.Minute {
				t.Errorf("cache write TTL = %v, want 1m", cache.setTTL)
			}
		})
	}
}

func TestCachedStoreGetPrimaryFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "missing record",
			err:  errURLNotFound,
		},
		{
			name: "storage failure",
			err:  errors.New("PostgreSQL unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &stubPrimaryStore{getErr: tt.err}
			cache := &stubCache{getErr: errCacheMiss}
			store := &cachedStore{
				primary: primary,
				cache:   cache,
				maxTTL:  time.Minute,
			}

			got, err := store.get(context.Background(), "go-guide")
			if !errors.Is(err, tt.err) {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}
			if got != (urlRecord{}) {
				t.Errorf("record = %+v, want empty record", got)
			}

			if cache.getCalls != 1 {
				t.Errorf("cache lookups = %d, want 1", cache.getCalls)
			}
			if primary.getCalls != 1 {
				t.Errorf("primary lookups = %d, want 1", primary.getCalls)
			}
			if cache.setCalls != 0 {
				t.Errorf("cache writes = %d, want 0", cache.setCalls)
			}
		})
	}
}

func TestCachedStoreGetCanceledContext(t *testing.T) {
	primary := &stubPrimaryStore{}
	cache := &stubCache{}
	store := &cachedStore{
		primary: primary,
		cache:   cache,
		maxTTL:  time.Minute,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := store.get(ctx, "go-guide")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}

	if cache.getCalls != 0 {
		t.Errorf("cache lookups = %d, want 0", cache.getCalls)
	}
	if primary.getCalls != 0 {
		t.Errorf("primary lookups = %d, want 0", primary.getCalls)
	}
	if cache.setCalls != 0 {
		t.Errorf("cache writes = %d, want 0", cache.setCalls)
	}
}

func TestCachedStoreSave(t *testing.T) {
	want := urlRecord{
		Code:        "go-guide",
		OriginalURL: "https://example.com/guide",
	}

	tests := []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "collision", err: errCodeExists},
		{
			name: "storage failure",
			err:  errors.New("PostgreSQL unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primary := &stubPrimaryStore{saveErr: tt.err}
			cache := &stubCache{}
			store := &cachedStore{
				primary: primary,
				cache:   cache,
				maxTTL:  time.Minute,
			}

			err := store.save(context.Background(), want)

			if tt.err == nil {
				if err != nil {
					t.Fatalf("save: %v", err)
				}
			} else if !errors.Is(err, tt.err) {
				t.Fatalf("error = %v, want %v", err, tt.err)
			}

			if primary.saveCalls != 1 {
				t.Errorf("primary saves = %d, want 1", primary.saveCalls)
			}
			if primary.saveRecord != want {
				t.Errorf("saved record = %+v, want %+v", primary.saveRecord, want)
			}
			if cache.getCalls != 0 {
				t.Errorf("cache lookups = %d, want 0", cache.getCalls)
			}
			if cache.setCalls != 0 {
				t.Errorf("cache writes = %d, want 0", cache.setCalls)
			}
		})
	}
}

func TestCachedStoreDelete(t *testing.T) {
	cacheErr := errors.New("Redis unavailable")
	primaryErr := errors.New("PostgreSQL unavailable")

	tests := []struct {
		name       string
		cacheErr   error
		primaryErr error
		wantErr    error
		wantOrder  []string
	}{
		{
			name:      "success",
			wantOrder: []string{"cache", "primary"},
		},
		{
			name:      "invalidation failure",
			cacheErr:  cacheErr,
			wantErr:   cacheErr,
			wantOrder: []string{"cache"},
		},
		{
			name:       "primary failure",
			primaryErr: primaryErr,
			wantErr:    primaryErr,
			wantOrder:  []string{"cache", "primary"},
		},
		{
			name:       "missing primary record",
			primaryErr: errURLNotFound,
			wantErr:    errURLNotFound,
			wantOrder:  []string{"cache", "primary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const code = "go-guide"
			var order []string

			primary := &stubPrimaryStore{
				deleteFunc: func(ctx context.Context, gotCode string) error {
					order = append(order, "primary")
					if gotCode != code {
						t.Errorf("primary code = %q, want %q", gotCode, code)
					}
					return tt.primaryErr
				},
			}

			cache := &stubCache{
				deleteFunc: func(ctx context.Context, gotCode string) error {
					order = append(order, "cache")
					if gotCode != code {
						t.Errorf("cache code = %q, want %q", gotCode, code)
					}
					return tt.cacheErr
				},
			}

			store := &cachedStore{
				primary: primary,
				cache:   cache,
				maxTTL:  time.Minute,
			}

			err := store.delete(context.Background(), code)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("delete: %v", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}

			if !slices.Equal(order, tt.wantOrder) {
				t.Errorf("operation order = %v, want %v", order, tt.wantOrder)
			}
		})
	}
}
