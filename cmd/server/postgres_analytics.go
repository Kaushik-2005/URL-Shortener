package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func (s *postgresStore) recordClick(
	ctx context.Context,
	urlID int64,
	at time.Time,
) error {
	const query = `
		INSERT INTO public.url_daily_clicks AS daily
			(url_id, click_date, click_count)
		VALUES ($1, $2::date, 1)
		ON CONFLICT (url_id, click_date)
		DO UPDATE SET click_count = daily.click_count + 1
	`

	_, err := s.pool.Exec(ctx, query, urlID, analyticsDate(at))
	if err != nil {
		var postgresErr *pgconn.PgError
		if errors.As(err, &postgresErr) &&
			postgresErr.Code == "23503" &&
			postgresErr.ConstraintName == "url_daily_clicks_url_id_fkey" {
			return errURLNotFound
		}

		return fmt.Errorf("record click: %w", err)
	}

	return nil
}

func (s *postgresStore) getClickStatistics(
	ctx context.Context,
	urlID int64,
) (clickStatistics, error) {
	const query = `
		SELECT
			to_char(daily.click_date, 'YYYY-MM-DD'),
			COALESCE(daily.click_count, 0)
		FROM public.urls AS urls
		LEFT JOIN public.url_daily_clicks AS daily
			ON daily.url_id = urls.id
		WHERE urls.id = $1
		ORDER BY daily.click_date
	`

	rows, err := s.pool.Query(ctx, query, urlID)
	if err != nil {
		return clickStatistics{}, fmt.Errorf("query click statistics: %w", err)
	}
	defer rows.Close()

	stats := clickStatistics{
		Days: make([]dailyClickCount, 0),
	}
	found := false

	for rows.Next() {
		found = true

		var date *string
		var count int64

		if err := rows.Scan(&date, &count); err != nil {
			return clickStatistics{}, fmt.Errorf("scan click statistics: %w", err)
		}

		if date == nil {
			continue
		}

		stats.Total += count
		stats.Days = append(stats.Days, dailyClickCount{
			Date:  *date,
			Count: count,
		})
	}

	if err := rows.Err(); err != nil {
		return clickStatistics{}, fmt.Errorf("read click statistics: %w", err)
	}
	if !found {
		return clickStatistics{}, errURLNotFound
	}

	return stats, nil
}

var _ clickRecorder = (*postgresStore)(nil)
var _ clickStatisticsReader = (*postgresStore)(nil)
