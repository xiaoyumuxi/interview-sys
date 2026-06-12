package workqueue

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type Stream struct {
	client *redis.Client
	logger *slog.Logger
	name   string
}

type Event struct {
	Type      string
	SessionID string
	Payload   any
}

func NewStream(client *redis.Client, logger *slog.Logger, name string) *Stream {
	return &Stream{client: client, logger: logger, name: name}
}

func (s *Stream) Publish(ctx context.Context, event Event) {
	if s == nil || s.client == nil {
		return
	}
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		s.logger.Warn("marshal stream event failed", "event_type", event.Type, "error", err)
		return
	}
	cmd := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.name,
		MaxLen: 10000,
		Approx: true,
		Values: map[string]any{
			"event_type": event.Type,
			"session_id": event.SessionID,
			"payload":    string(payload),
			"created_at": time.Now().Format(time.RFC3339Nano),
		},
	})
	if err := cmd.Err(); err != nil {
		s.logger.Warn("publish stream event failed", "stream", s.name, "event_type", event.Type, "error", err)
	}
}
