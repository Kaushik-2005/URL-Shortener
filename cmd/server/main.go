package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	mux := http.NewServeMux()
	var store urlStore
	var statisticsReader clickStatisticsReader
	var readinessCheck func(context.Context) error
	var recorder clickRecorder

	switch backend := os.Getenv("STORAGE_BACKEND"); backend {
	case "", "memory":
		memory := newMemoryStore()
		store = memory
		statisticsReader = memory
		recorder = memory
		readinessCheck = func(context.Context) error {
			return nil
		}

	case "postgres":
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			return errors.New("DATABASE_URL is required for PostgreSQL storage")
		}

		pool, err := openPostgres(context.Background(), databaseURL)
		if err != nil {
			return fmt.Errorf("Open PostgreSQL: %w", err)
		}
		defer pool.Close()

		postgres := &postgresStore{pool: pool}
		store = postgres
		statisticsReader = postgres
		recorder = postgres
		readinessCheck = pool.Ping

	default:
		return fmt.Errorf("unknown STORAGE_BACKEND: %q", backend)
	}

	primaryStore := store

	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		if os.Getenv("STORAGE_BACKEND") != "postgres" {
			return errors.New("REDIS_URL requires STORAGE_BACKEND=postgres")
		}

		client, err := newRedisClient(redisURL)
		if err != nil {
			return fmt.Errorf("Configure Redis: %w", err)
		}

		defer func() {
			if err := client.Close(); err != nil {
				logger.Warn("Close Redis failed", "error", err)
			}
		}()

		cache := &redisCache{
			client:    client,
			namespace: rand.Text(),
		}

		store = &cachedStore{
			primary: store,
			cache:   cache,
			maxTTL:  5 * time.Minute,
		}

		logger.Info("Redis lookup caching configured")
	}

	managementKey := os.Getenv("MANAGEMENT_API_KEY")
	if managementKey != "" {
		deleteHandler, err := requireAPIKey(
			managementKey,
			newDeleteURLHandler(store),
		)
		if err != nil {
			return fmt.Errorf("Configure management authentication: %w", err)
		}

		statisticsHandler, err := requireAPIKey(
			managementKey,
			newStatisticsHandler(primaryStore, statisticsReader),
		)
		if err != nil {
			return fmt.Errorf("configure statistics authentication: %w", err)
		}

		mux.Handle(
			"GET /api/v1/urls/{code}/stats",
			statisticsHandler,
		)

		mux.Handle(
			"DELETE /api/v1/urls/{code}",
			deleteHandler,
		)
	}

	baseURL := "http://127.0.0.1:8080"

	createLimiter, err := newCreateLimiter(
		10,
		time.Minute,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("configure creation rate limit: %w", err)
	}

	mux.Handle(
		"POST /api/v1/urls",
		withCreateRateLimit(
			createLimiter,
			time.Now,
			newCreateURLHandler(store, baseURL, generateCode),
		),
	)

	mux.HandleFunc("GET /{code}", newRedirectHandler(store, recorder))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "OK")
	})

	mux.HandleFunc("GET /readyz", newReadinessHandler(readinessCheck))

	mux.HandleFunc(
		"GET /api/v1/urls/{code}",
		newMetadataHandler(store),
	)

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "Hello, Go!")
	})

	server := &http.Server{
		Addr: "127.0.0.1:8080",
		Handler: withRequestID(
			withRequestTimeout(
				5*time.Second,
				withRequestLogging(logger, mux),
			),
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	logger.Info("Starting server", "address", server.Addr)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-signalCtx.Done():
		logger.Info("Shutdown requested")
	}

	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("Graceful shutdown failed: %w", err)
	}
	logger.Info("Server stopped cleanly")

	return nil
}
