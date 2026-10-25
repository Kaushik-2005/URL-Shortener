package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStore struct {
	pool *pgxpool.Pool
}

func (s *postgresStore) save(
	ctx context.Context,
	record urlRecord,
) error {
	const query = `
		INSERT INTO public.urls (code, original_url)
		VALUES ($1, $2)
	`

	_, err := s.pool.Exec(
		ctx,
		query,
		record.Code,
		record.OriginalURL,
	)
	if err != nil {
		var postgresErr *pgconn.PgError
		if errors.As(err, &postgresErr) &&
			postgresErr.Code == "23505" &&
			postgresErr.ConstraintName == "urls_code_unique" {
			return errCodeExists
		}

		return fmt.Errorf("insert URL: %w", err)
	}

	return nil
}

func (s *postgresStore) get(
	ctx context.Context,
	code string,
) (urlRecord, error) {
	const query = `
		SELECT code, original_url
		FROM public.urls
		WHERE code = $1
	`

	var record urlRecord
	err := s.pool.QueryRow(ctx, query, code).Scan(
		&record.Code,
		&record.OriginalURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return urlRecord{}, errURLNotFound
	}
	if err != nil {
		return urlRecord{}, fmt.Errorf("get URL: %w", err)
	}

	return record, nil
}
