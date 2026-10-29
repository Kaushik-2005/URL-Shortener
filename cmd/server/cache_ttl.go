package main

import "time"

func calculateCacheTTL(
	record urlRecord,
	maxTTL time.Duration,
	now time.Time,
) time.Duration {
	if maxTTL <= 0 {
		return 0
	}

	ttl := maxTTL

	if record.ExpiresAt != nil {
		remaining := record.ExpiresAt.Sub(now)
		if remaining <= 0 {
			return 0
		}

		if remaining < ttl {
			ttl = remaining
		}
	}

	return ttl.Truncate(time.Millisecond)
}
