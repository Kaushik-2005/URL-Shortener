package main

import (
	"context"
	"time"
)

type dailyClickCount struct {
	Date  string
	Count int64
}

type clickStatistics struct {
	Total int64
	Days  []dailyClickCount
}

type clickRecorder interface {
	recordClick(ctx context.Context, urlID int64, at time.Time) error
}

type clickStatisticsReader interface {
	getClickStatistics(ctx context.Context, urlID int64) (clickStatistics, error)
}

func analyticsDate(at time.Time) string {
	return at.UTC().Format(time.DateOnly)
}
