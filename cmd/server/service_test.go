package main

import (
	"errors"
	"testing"
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

	got, err := service.create(want.OriginalURL)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if got != want {
		t.Errorf("created record = %+v, want %+v", got, want)
	}

	saved, exists := repository.get(want.Code)
	if !exists {
		t.Fatal("created record was not saved")
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

			got, err := service.create(tt.url)

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

	got, err := service.create("https://example.com")

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

	got, err := service.create("https://example.com")

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

	if err := repository.save(original); err != nil {
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

	got, err := service.create(want.OriginalURL)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if got != want {
		t.Errorf("created record = %+v, want %+v", got, want)
	}

	if generatorCalls != 2 {
		t.Errorf("generator calls = %d, want 2", generatorCalls)
	}

	savedOriginal, exists := repository.get(original.Code)
	if !exists || savedOriginal != original {
		t.Errorf(
			"original record = (%+v, %t), want (%+v, true)",
			savedOriginal, exists, original,
		)
	}

	savedNew, exists := repository.get(want.Code)
	if !exists || savedNew != want {
		t.Errorf(
			"new record = (%+v, %t), want (%+v, true)",
			savedNew, exists, want,
		)
	}
}

func TestURLServiceCreateCollisionLimit(t *testing.T) {
	repository := newMemoryStore()
	original := urlRecord{
		Code:        "taken123",
		OriginalURL: "https://example.com/original",
	}

	if err := repository.save(original); err != nil {
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

	got, err := service.create("https://example.com/new")

	if !errors.Is(err, errCodeAllocationExhausted) {
		t.Fatalf("error = %v, want errCodeAllocationExhausted", err)
	}

	if got != (urlRecord{}) {
		t.Errorf("record = %+v, want empty record", got)
	}

	if generatorCalls != 5 {
		t.Errorf("generator calls = %d, want 5", generatorCalls)
	}

	saved, exists := repository.get(original.Code)
	if !exists || saved != original {
		t.Errorf(
			"original record = (%+v, %t), want (%+v, true)",
			saved, exists, original,
		)
	}

	repository.mu.RLock()
	count := len(repository.urls)
	repository.mu.RUnlock()

	if count != 1 {
		t.Errorf("stored records = %d, want 1", count)
	}
}
