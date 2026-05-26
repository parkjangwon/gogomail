package maildb

import (
	"database/sql"
	"time"
	"unicode/utf8"
)

func optionalTimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func optionalTimeStringPtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullableTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func applyExportBatchNullableTimes(batch *APIUsageExportBatchView, completedAt sql.NullTime, windowStart sql.NullTime, windowEnd sql.NullTime, firstEventAt sql.NullTime, lastEventAt sql.NullTime) {
	if completedAt.Valid {
		batch.CompletedAt = &completedAt.Time
	}
	if windowStart.Valid {
		batch.WindowStart = &windowStart.Time
	}
	if windowEnd.Valid {
		batch.WindowEnd = &windowEnd.Time
	}
	if firstEventAt.Valid {
		batch.FirstEventAt = &firstEventAt.Time
	}
	if lastEventAt.Valid {
		batch.LastEventAt = &lastEventAt.Time
	}
}

func quotaUsageRatio(used int64, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	if used <= 0 {
		return 0
	}
	return float64(used) / float64(limit)
}

func quotaRemaining(used int64, limit int64) int64 {
	if limit <= 0 {
		return 0
	}
	remaining := limit - used
	if remaining < 0 {
		return 0
	}
	return remaining
}
