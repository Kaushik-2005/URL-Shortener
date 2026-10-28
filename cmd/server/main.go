package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	mux := http.NewServeMux()
	var store urlStore

	switch backend := os.Getenv("STORAGE_BACKEND"); backend {
	case "", "memory":
		store = newMemoryStore()

	case "postgres":
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			log.Fatal("DATABASE_URL is required for PostgreSQL storage")
		}

		pool, err := openPostgres(context.Background(), databaseURL)
		if err != nil {
			log.Fatalf("Open PostgreSQL: %v", err)
		}
		defer pool.Close()

		store = &postgresStore{pool: pool}

	default:
		log.Fatalf("Unknown STORAGE_BACKEND: %q", backend)
	}

	managementKey := os.Getenv("MANAGEMENT_API_KEY")
	if managementKey != "" {
		deleteHandler, err := requireAPIKey(
			managementKey,
			newDeleteURLHandler(store),
		)
		if err != nil {
			log.Fatalf("Configure management authentication: %v", err)
		}

		mux.Handle(
			"DELETE /api/v1/urls/{code}",
			deleteHandler,
		)
	}

	baseURL := "http://127.0.0.1:8080"

	mux.HandleFunc(
		"POST /api/v1/urls",
		newCreateURLHandler(store, baseURL, generateCode),
	)

	mux.HandleFunc("GET /{code}", newRedirectHandler(store))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "OK")
	})

	mux.HandleFunc(
		"GET /api/v1/urls/{code}",
		newMetadataHandler(store),
	)

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "Hello, Go!")
	})

	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	log.Printf("Starting server on http://%s", server.Addr)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
		return
	case <-signalCtx.Done():
		log.Println("Shutdown requested")
	}

	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}
	log.Println("Server stopped cleanly")
}
