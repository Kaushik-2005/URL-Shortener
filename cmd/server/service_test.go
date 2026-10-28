package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestURLServiceCreateSuccess(t *testing.T) {
	repository := newMemoryStore()

	generatorCalls := 0
	generate := func() (string, error) {
		generatorCalls++
		return "abc12345", nil
	}

	service := &urlService{
		repository: repository,
		generate:   generate,
	}

	want := urlRecord{
		Code:        "abc12345",
		OriginalURL: "https://example.com/articles/go",
	}

	got, err := service.create(context.Background(), want.OriginalURL, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if got != want {
		t.Errorf("created record = %+v, want %+v", got, want)
	}

	saved, lookupErr := repository.get(context.Background(), want.Code)
	if lookupErr != nil {
		t.Fatalf("get saved record: %v", lookupErr)
	}

	if saved != want {
		t.Errorf("saved record = %+v, want %+v", saved, want)
	}

	if generatorCalls != 1 {
		t.Errorf("generator calls = %d, want 1", generatorCalls)
	}
}

func TestURLServiceCreateRejectsInvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "empty URL",
			url:  "",
		},
		{
			name: "relative path",
			url:  "/articles/go",
		},
		{
			name: "unsupported scheme",
			url:  "ftp://example.com",
		},
		{
			name: "missing hostname",
			url:  "https:///articles/go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := newMemoryStore()
			generatorCalls := 0

			service := &urlService{
				repository: repository,
				generate: func() (string, error) {
					generatorCalls++
					return "abc12345", nil
				},
			}

			got, err := service.create(context.Background(), tt.url, nil)

			if !errors.Is(err, errInvalidURL) {
				t.Fatalf("error = %v, want errInvalidURL", err)
			}

			if got != (urlRecord{}) {
				t.Errorf("record = %+v, want empty record", got)
			}

			if generatorCalls != 0 {
				t.Errorf("generator calls = %d, want 0", generatorCalls)
			}

			repository.mu.RLock()
			count := len(repository.urls)
			repository.mu.RUnlock()

			if count != 0 {
				t.Errorf("stored records = %d, want 0", count)
			}
		})
	}
}

func TestURLServiceCreateGeneratorFailure(t *testing.T) {
	repository := newMemoryStore()
	generatorErr := errors.New("random source unavailable")
	generatorCalls := 0

	service := &urlService{
		repository: repository,
		generate: func() (string, error) {
			generatorCalls++
			return "", generatorErr
		},
	}

	got, err := service.create(context.Background(), "https://example.com", nil)

	if !errors.Is(err, generatorErr) {
		t.Fatalf("error = %v, want wrapped generator error", err)
	}

	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}

	if generatorCalls != 1 {
		t.Errorf("generator calls = %d, want 1", generatorCalls)
	}

	repository.mu.RLock()
	count := len(repository.urls)
	repository.mu.RUnlock()

	if count != 0 {
		t.Errorf("stored records = %d, want 0", count)
	}
}

func TestURLServiceCreateRepositoryFailure(t *testing.T) {
	repositoryErr := errors.New("storage unavailable")
	repository := &failingSaveRepository{
		saveErr: repositoryErr,
	}

	generatorCalls := 0
	service := &urlService{
		repository: repository,
		generate: func() (string, error) {
			generatorCalls++
			return "abc12345", nil
		},
	}

	got, err := service.create(context.Background(), "https://example.com", nil)

	if !errors.Is(err, repositoryErr) {
		t.Fatalf("error = %v, want wrapped repository error", err)
	}

	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}

	if repository.saveCalls != 1 {
		t.Errorf("save calls = %d, want 1", repository.saveCalls)
	}

	if generatorCalls != 1 {
		t.Errorf("generator calls = %d, want 1", generatorCalls)
	}
}

func TestURLServiceCreateRetriesCollision(t *testing.T) {
	repository := newMemoryStore()
	original := urlRecord{
		Code:        "taken123",
		OriginalURL: "https://example.com/original",
	}

	if err := repository.save(context.Background(), original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	generatorCalls := 0
	service := &urlService{
		repository: repository,
		generate: func() (string, error) {
			generatorCalls++
			if generatorCalls == 1 {
				return original.Code, nil
			}
			return "fresh456", nil
		},
	}

	want := urlRecord{
		Code:        "fresh456",
		OriginalURL: "https://example.com/new",
	}

	got, err := service.create(context.Background(), want.OriginalURL, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if got != want {
		t.Errorf("created record = %+v, want %+v", got, want)
	}

	if generatorCalls != 2 {
		t.Errorf("generator calls = %d, want 2", generatorCalls)
	}

	savedOriginal, lookupErr := repository.get(
		context.Background(),
		original.Code,
	)
	if lookupErr != nil {
		t.Fatalf("get original record: %v", lookupErr)
	}

	if savedOriginal != original {
		t.Errorf("original record = %+v, want %+v", savedOriginal, original)
	}

	savedNew, lookupErr := repository.get(context.Background(), want.Code)
	if lookupErr != nil {
		t.Fatalf("get new record: %v", lookupErr)
	}

	if savedNew != want {
		t.Errorf("new record = %+v, want %+v", savedNew, want)
	}
}

func TestURLServiceCreateCollisionLimit(t *testing.T) {
	repository := newMemoryStore()
	original := urlRecord{
		Code:        "taken123",
		OriginalURL: "https://example.com/original",
	}

	if err := repository.save(context.Background(), original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	generatorCalls := 0
	service := &urlService{
		repository: repository,
		generate: func() (string, error) {
			generatorCalls++
			return original.Code, nil
		},
	}

	got, err := service.create(context.Background(), "https://example.com/new", nil)

	if !errors.Is(err, errCodeAllocationExhausted) {
		t.Fatalf("error = %v, want errCodeAllocationExhausted", err)
	}

	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}

	if generatorCalls != 5 {
		t.Errorf("generator calls = %d, want 5", generatorCalls)
	}

	saved, lookupErr := repository.get(context.Background(), original.Code)
	if lookupErr != nil {
		t.Fatalf("get original after retry exhaustion: %v", lookupErr)
	}

	if saved != original {
		t.Errorf("original record = %+v, want %+v", saved, original)
	}

	repository.mu.RLock()
	count := len(repository.urls)
	repository.mu.RUnlock()

	if count != 1 {
		t.Errorf("stored records = %d, want 1", count)
	}
}

func TestURLServiceCreateWithAliasSuccess(t *testing.T) {
	repository := newMemoryStore()
	generatorCalls := 0

	service := &urlService{
		repository: repository,
		generate: func() (string, error) {
			generatorCalls++
			return "random12", nil
		},
	}

	want := urlRecord{
		Code:        "Go-2026",
		OriginalURL: "https://example.com/articles/go",
	}

	got, err := service.createWithAlias(
		context.Background(),
		want.OriginalURL,
		want.Code,
		nil,
	)
	if err != nil {
		t.Fatalf("create with alias: %v", err)
	}

	if got != want {
		t.Errorf("created record = %+v, want %+v", got, want)
	}

	saved, lookupErr := repository.get(
		context.Background(),
		want.Code,
	)
	if lookupErr != nil {
		t.Fatalf("get saved record: %v", lookupErr)
	}

	if saved != want {
		t.Errorf("saved record = %+v, want %+v", saved, want)
	}
	if generatorCalls != 0 {
		t.Errorf("generator calls = %d, want 0", generatorCalls)
	}
}

func TestURLServiceCreateWithAliasConflict(t *testing.T) {
	repository := newMemoryStore()
	original := urlRecord{
		Code:        "go-guide",
		OriginalURL: "https://example.com/original",
	}

	if err := repository.save(context.Background(), original); err != nil {
		t.Fatalf("save original: %v", err)
	}

	generatorCalls := 0
	service := &urlService{
		repository: repository,
		generate: func() (string, error) {
			generatorCalls++
			return "random12", nil
		},
	}

	got, err := service.createWithAlias(
		context.Background(),
		"https://example.com/replacement",
		original.Code,
		nil,
	)

	if !errors.Is(err, errAliasTaken) {
		t.Fatalf("error = %v, want errAliasTaken", err)
	}

	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}

	saved, lookupErr := repository.get(
		context.Background(),
		original.Code,
	)
	if lookupErr != nil {
		t.Fatalf("get original: %v", lookupErr)
	}

	if saved != original {
		t.Errorf("original record = %+v, want %+v", saved, original)
	}

	if generatorCalls != 0 {
		t.Errorf("generator calls = %d, want 0", generatorCalls)
	}

	repository.mu.RLock()
	count := len(repository.urls)
	repository.mu.RUnlock()

	if count != 1 {
		t.Errorf("stored records = %d, want 1", count)
	}
}

func TestURLServiceCreateWithAliasRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		alias   string
		wantErr error
	}{
		{
			name:    "invalid URL",
			url:     "ftp://example.com",
			alias:   "go-guide",
			wantErr: errInvalidURL,
		},
		{
			name:    "empty alias",
			url:     "https://example.com",
			alias:   "",
			wantErr: errInvalidAlias,
		},
		{
			name:    "reserved alias",
			url:     "https://example.com",
			alias:   "healthz",
			wantErr: errInvalidAlias,
		},
		{
			name:    "alias with slash",
			url:     "https://example.com",
			alias:   "go/guide",
			wantErr: errInvalidAlias,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &failingSaveRepository{
				saveErr: errors.New("unexpected save attempt"),
			}
			generatorCalls := 0

			service := &urlService{
				repository: repository,
				generate: func() (string, error) {
					generatorCalls++
					return "random12", nil
				},
			}

			got, err := service.createWithAlias(
				context.Background(),
				tt.url,
				tt.alias,
				nil,
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}

			if got != (urlRecord{}) {
				t.Errorf("record = %+v, want empty record", got)
			}

			if repository.saveCalls != 0 {
				t.Errorf("save calls = %d, want 0", repository.saveCalls)
			}

			if generatorCalls != 0 {
				t.Errorf("generator calls = %d, want 0", generatorCalls)
			}
		})
	}
}

func TestURLServiceCreateWithAliasRepositoryFailure(t *testing.T) {
	repositoryErr := errors.New("storage unavailable")
	repository := &failingSaveRepository{
		saveErr: repositoryErr,
	}
	generatorCalls := 0

	service := &urlService{
		repository: repository,
		generate: func() (string, error) {
			generatorCalls++
			return "random12", nil
		},
	}

	got, err := service.createWithAlias(
		context.Background(),
		"https://example.com",
		"go-guide",
		nil,
	)

	if !errors.Is(err, repositoryErr) {
		t.Fatalf("error = %v, want wrapped repository error", err)
	}

	if errors.Is(err, errAliasTaken) {
		t.Error("repository failure was classified as an alias conflict")
	}

	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}

	if repository.saveCalls != 1 {
		t.Errorf("save calls = %d, want 1", repository.saveCalls)
	}

	if generatorCalls != 0 {
		t.Errorf("generator calls = %d, want 0", generatorCalls)
	}
}

func TestURLServiceCreateWithExpiry(t *testing.T) {
	tests := []struct {
		name     string
		useAlias bool
	}{
		{name: "random code"},
		{name: "custom alias", useAlias: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := newMemoryStore()
			service := &urlService{
				repository: repository,
				generate: func() (string, error) {
					return "random12", nil
				},
			}

			expiry := time.Now().UTC().Truncate(time.Second).Add(
				time.Hour + 123456789*time.Nanosecond,
			).In(time.FixedZone("UTC+0530", 5*3600+30*60))

			wantExpiry := expiry.UTC().Truncate(time.Microsecond)

			var got urlRecord
			var err error

			if tt.useAlias {
				got, err = service.createWithAlias(
					context.Background(),
					"https://example.com/expiry",
					"go-guide",
					&expiry,
				)
			} else {
				got, err = service.create(
					context.Background(),
					"https://example.com/expiry",
					&expiry,
				)
			}

			if err != nil {
				t.Fatalf("create: %v", err)
			}

			if got.ExpiresAt == nil {
				t.Fatal("returned expiry is nil")
			}

			if !got.ExpiresAt.Equal(wantExpiry) {
				t.Errorf(
					"returned expiry = %v, want %v",
					*got.ExpiresAt, wantExpiry,
				)
			}

			saved, lookupErr := repository.get(
				context.Background(),
				got.Code,
			)
			if lookupErr != nil {
				t.Fatalf("get saved record: %v", lookupErr)
			}

			if saved.ExpiresAt == nil {
				t.Fatal("stored expiry is nil")
			}

			if !saved.ExpiresAt.Equal(wantExpiry) {
				t.Errorf(
					"stored expiry = %v, want %v",
					*saved.ExpiresAt, wantExpiry,
				)
			}
		})
	}
}

func TestURLServiceRejectsPastExpiry(t *testing.T) {
	tests := []struct {
		name     string
		useAlias bool
	}{
		{name: "random code"},
		{name: "custom alias", useAlias: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &failingSaveRepository{
				saveErr: errors.New("unexpected save attempt"),
			}
			generatorCalls := 0

			service := &urlService{
				repository: repository,
				generate: func() (string, error) {
					generatorCalls++
					return "random12", nil
				},
			}

			expiry := time.Now().Add(-time.Hour)

			var got urlRecord
			var err error

			if tt.useAlias {
				got, err = service.createWithAlias(
					context.Background(),
					"https://example.com",
					"go-guide",
					&expiry,
				)
			} else {
				got, err = service.create(
					context.Background(),
					"https://example.com",
					&expiry,
				)
			}

			if !errors.Is(err, errInvalidExpiry) {
				t.Fatalf("error = %v, want errInvalidExpiry", err)
			}

			if got != (urlRecord{}) {
				t.Errorf("record = %+v, want empty record", got)
			}

			if repository.saveCalls != 0 {
				t.Errorf("save calls = %d, want 0", repository.saveCalls)
			}

			if generatorCalls != 0 {
				t.Errorf("generator calls = %d, want 0", generatorCalls)
			}
		})
	}
}
