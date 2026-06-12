package workqueue

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
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

func (s *Stream) EnsureGroup(ctx context.Context, group string) {
	if s == nil || s.client == nil || group == "" {
		return
	}
	err := s.client.XGroupCreateMkStream(ctx, s.name, group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		s.logger.Warn("ensure stream group failed", "stream", s.name, "group", group, "error", err)
	}
}

func (s *Stream) ReadGroup(ctx context.Context, group string, consumer string, count int, block time.Duration) ([]redis.XMessage, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("stream is unavailable")
	}
	streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{s.name, ">"},
		Count:    int64(count),
		Block:    block,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

func (s *Stream) Ack(ctx context.Context, group string, ids ...string) {
	if s == nil || s.client == nil || len(ids) == 0 {
		return
	}
	if err := s.client.XAck(ctx, s.name, group, ids...).Err(); err != nil {
		s.logger.Warn("ack stream messages failed", "stream", s.name, "group", group, "error", err)
	}
}
