package main

import (
	"context"
	"sort"
	"time"
)

func (s *memoryStore) recordClick(
	ctx context.Context,
	urlID int64,
	at time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	days, exists := s.clicks[urlID]
	if !exists {
		return errURLNotFound
	}

	days[analyticsDate(at)]++
	return nil
}

func (s *memoryStore) getClickStatistics(
	ctx context.Context,
	urlID int64,
) (clickStatistics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := ctx.Err(); err != nil {
		return clickStatistics{}, err
	}

	days, exists := s.clicks[urlID]
	if !exists {
		return clickStatistics{}, errURLNotFound
	}

	stats := clickStatistics{
		Days: make([]dailyClickCount, 0, len(days)),
	}

	for date, count := range days {
		stats.Total += count
		stats.Days = append(stats.Days, dailyClickCount{
			Date:  date,
			Count: count,
		})
	}

	sort.Slice(stats.Days, func(i, j int) bool {
		return stats.Days[i].Date < stats.Days[j].Date
	})

	return stats, nil
}

var _ clickRecorder = (*memoryStore)(nil)
var _ clickStatisticsReader = (*memoryStore)(nil)
