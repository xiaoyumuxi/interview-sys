package store

import (
	"context"
	"database/sql"
	"time"
)

type AsyncMessageSummary struct {
	Total           int            `json:"total"`
	ByStatus        map[string]int `json:"by_status"`
	OldestPendingAt string         `json:"oldest_pending_at,omitempty"`
	NewestAt        string         `json:"newest_at,omitempty"`
}

func (s *Store) AsyncMessageSummary(ctx context.Context) (AsyncMessageSummary, error) {
	summary := AsyncMessageSummary{ByStatus: map[string]int{}}
	rows, err := s.db.QueryContext(ctx, `SELECT status, count(*) FROM async_messages GROUP BY status`)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			_ = rows.Close()
			return summary, err
		}
		summary.ByStatus[status] = count
		summary.Total += count
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}
	var oldestPending, newest sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
SELECT min(next_retry_at) FILTER (WHERE status IN ('pending','failed','dispatching')), max(updated_at)
FROM async_messages`).Scan(&oldestPending, &newest); err != nil {
		return summary, err
	}
	if oldestPending.Valid {
		summary.OldestPendingAt = oldestPending.Time.Format(time.RFC3339)
	}
	if newest.Valid {
		summary.NewestAt = newest.Time.Format(time.RFC3339)
	}
	return summary, nil
}
