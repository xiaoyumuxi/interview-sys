package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type DeadLetterEvent struct {
	DeadLetterID    string         `json:"dead_letter_id"`
	Source          string         `json:"source"`
	SourceStream    string         `json:"source_stream"`
	SourceMessageID string         `json:"source_message_id"`
	EventType       string         `json:"event_type"`
	AggregateType   string         `json:"aggregate_type"`
	AggregateID     string         `json:"aggregate_id"`
	Payload         map[string]any `json:"payload"`
	Reason          string         `json:"reason"`
	ErrorText       string         `json:"error_text"`
	Attempts        int            `json:"attempts"`
	Status          string         `json:"status"`
	OccurrenceCount int            `json:"occurrence_count"`
	FirstSeenAt     string         `json:"first_seen_at"`
	LastSeenAt      string         `json:"last_seen_at"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

type DeadLetterUpsert struct {
	Source          string
	SourceStream    string
	SourceMessageID string
	EventType       string
	AggregateType   string
	AggregateID     string
	Payload         any
	Reason          string
	ErrorText       string
	Attempts        int
}

type DeadLetterSummary struct {
	Total     int            `json:"total"`
	ByStatus  map[string]int `json:"by_status"`
	BySource  map[string]int `json:"by_source"`
	NewestAt  string         `json:"newest_at,omitempty"`
	OldestNew string         `json:"oldest_new,omitempty"`
}

func (s *Store) UpsertDeadLetter(ctx context.Context, item DeadLetterUpsert) error {
	source := strings.TrimSpace(item.Source)
	if source == "" {
		source = "redis_stream"
	}
	payloadRaw, err := json.Marshal(item.Payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO dead_letter_events (
  dead_letter_id, source, source_stream, source_message_id, event_type,
  aggregate_type, aggregate_id, payload, reason, error_text, attempts, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())
ON CONFLICT (source, source_stream, source_message_id) DO UPDATE SET
  payload=EXCLUDED.payload,
  reason=EXCLUDED.reason,
  error_text=EXCLUDED.error_text,
  attempts=GREATEST(dead_letter_events.attempts, EXCLUDED.attempts),
  occurrence_count=dead_letter_events.occurrence_count+1,
  last_seen_at=now(),
  updated_at=now()`,
		NewID("dlq"),
		source,
		strings.TrimSpace(item.SourceStream),
		strings.TrimSpace(item.SourceMessageID),
		strings.TrimSpace(item.EventType),
		strings.TrimSpace(item.AggregateType),
		strings.TrimSpace(item.AggregateID),
		payloadRaw,
		strings.TrimSpace(item.Reason),
		strings.TrimSpace(item.ErrorText),
		item.Attempts,
	)
	return err
}

func (s *Store) ListDeadLetters(ctx context.Context, status string, source string, limit int) ([]DeadLetterEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT dead_letter_id, source, source_stream, source_message_id, event_type, aggregate_type, aggregate_id,
       payload, reason, error_text, attempts, status, occurrence_count, first_seen_at, last_seen_at, created_at, updated_at
FROM dead_letter_events
WHERE 1=1`
	args := []any{}
	if strings.TrimSpace(status) != "" {
		args = append(args, status)
		query += ` AND status=$` + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(source) != "" {
		args = append(args, source)
		query += ` AND source=$` + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	query += ` ORDER BY last_seen_at DESC LIMIT $` + strconv.Itoa(len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DeadLetterEvent
	for rows.Next() {
		item, err := scanDeadLetter(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetDeadLetter(ctx context.Context, deadLetterID string) (DeadLetterEvent, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT dead_letter_id, source, source_stream, source_message_id, event_type, aggregate_type, aggregate_id,
       payload, reason, error_text, attempts, status, occurrence_count, first_seen_at, last_seen_at, created_at, updated_at
FROM dead_letter_events
WHERE dead_letter_id=$1`, deadLetterID)
	item, err := scanDeadLetter(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return DeadLetterEvent{}, false, nil
		}
		return DeadLetterEvent{}, false, err
	}
	return item, true, nil
}

func (s *Store) DeadLetterSummary(ctx context.Context) (DeadLetterSummary, error) {
	summary := DeadLetterSummary{ByStatus: map[string]int{}, BySource: map[string]int{}}
	rows, err := s.db.QueryContext(ctx, `SELECT status, count(*) FROM dead_letter_events GROUP BY status`)
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
	rows, err = s.db.QueryContext(ctx, `SELECT source, count(*) FROM dead_letter_events GROUP BY source`)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			_ = rows.Close()
			return summary, err
		}
		summary.BySource[source] = count
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}
	var newest, oldestNew sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
SELECT max(last_seen_at), min(first_seen_at) FILTER (WHERE status='new')
FROM dead_letter_events`).Scan(&newest, &oldestNew); err != nil {
		return summary, err
	}
	if newest.Valid {
		summary.NewestAt = newest.Time.Format(time.RFC3339)
	}
	if oldestNew.Valid {
		summary.OldestNew = oldestNew.Time.Format(time.RFC3339)
	}
	return summary, nil
}

type deadLetterScanner interface {
	Scan(dest ...any) error
}

func scanDeadLetter(row deadLetterScanner) (DeadLetterEvent, error) {
	var item DeadLetterEvent
	var payloadRaw []byte
	var firstSeen, lastSeen, created, updated time.Time
	if err := row.Scan(
		&item.DeadLetterID,
		&item.Source,
		&item.SourceStream,
		&item.SourceMessageID,
		&item.EventType,
		&item.AggregateType,
		&item.AggregateID,
		&payloadRaw,
		&item.Reason,
		&item.ErrorText,
		&item.Attempts,
		&item.Status,
		&item.OccurrenceCount,
		&firstSeen,
		&lastSeen,
		&created,
		&updated,
	); err != nil {
		return DeadLetterEvent{}, err
	}
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &item.Payload)
	}
	if item.Payload == nil {
		item.Payload = map[string]any{}
	}
	item.FirstSeenAt = firstSeen.Format(time.RFC3339)
	item.LastSeenAt = lastSeen.Format(time.RFC3339)
	item.CreatedAt = created.Format(time.RFC3339)
	item.UpdatedAt = updated.Format(time.RFC3339)
	return item, nil
}
