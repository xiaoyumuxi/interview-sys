package workqueue

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type ConsumerGroupMetric struct {
	Name            string `json:"name"`
	Consumers       int64  `json:"consumers"`
	Pending         int64  `json:"pending"`
	LastDeliveredID string `json:"last_delivered_id"`
	EntriesRead     int64  `json:"entries_read"`
	Lag             int64  `json:"lag"`
}

type QueueMetrics struct {
	StreamName       string                `json:"stream_name"`
	DeadLetterName   string                `json:"dead_letter_name"`
	StreamLength     int64                 `json:"stream_length"`
	DeadLetterLength int64                 `json:"dead_letter_length"`
	Groups           []ConsumerGroupMetric `json:"groups"`
	DeadLetterGroups []ConsumerGroupMetric `json:"dead_letter_groups"`
}

func (s *Stream) Metrics(ctx context.Context, groupName string, deadLetterGroup string) (QueueMetrics, error) {
	if s == nil || s.client == nil {
		return QueueMetrics{}, errors.New("stream is unavailable")
	}
	metrics := QueueMetrics{
		StreamName:     s.name,
		DeadLetterName: s.deadLetterName,
	}
	streamLen, err := s.client.XLen(ctx, s.name).Result()
	if err != nil {
		return QueueMetrics{}, err
	}
	metrics.StreamLength = streamLen
	deadLen, err := s.client.XLen(ctx, s.deadLetterName).Result()
	if err != nil {
		return QueueMetrics{}, err
	}
	metrics.DeadLetterLength = deadLen
	if groups, err := s.client.XInfoGroups(ctx, s.name).Result(); err == nil {
		metrics.Groups = convertGroupMetrics(groups)
	}
	if groups, err := s.client.XInfoGroups(ctx, s.deadLetterName).Result(); err == nil {
		metrics.DeadLetterGroups = convertGroupMetrics(groups)
	}
	if groupName != "" {
		if consumers, err := s.client.XInfoConsumers(ctx, s.name, groupName).Result(); err == nil && len(consumers) > 0 {
			metrics.Groups = append(metrics.Groups, ConsumerGroupMetric{
				Name:        groupName,
				Consumers:   int64(len(consumers)),
				Pending:     pendingCount(consumers),
				EntriesRead: 0,
			})
		}
	}
	if deadLetterGroup != "" {
		if consumers, err := s.client.XInfoConsumers(ctx, s.deadLetterName, deadLetterGroup).Result(); err == nil && len(consumers) > 0 {
			metrics.DeadLetterGroups = append(metrics.DeadLetterGroups, ConsumerGroupMetric{
				Name:        deadLetterGroup,
				Consumers:   int64(len(consumers)),
				Pending:     pendingCount(consumers),
				EntriesRead: 0,
			})
		}
	}
	return metrics, nil
}

func convertGroupMetrics(groups []redis.XInfoGroup) []ConsumerGroupMetric {
	items := make([]ConsumerGroupMetric, 0, len(groups))
	for _, group := range groups {
		items = append(items, ConsumerGroupMetric{
			Name:            group.Name,
			Consumers:       group.Consumers,
			Pending:         group.Pending,
			LastDeliveredID: group.LastDeliveredID,
			EntriesRead:     group.EntriesRead,
			Lag:             group.Lag,
		})
	}
	return items
}

func pendingCount(consumers []redis.XInfoConsumer) int64 {
	var total int64
	for _, consumer := range consumers {
		total += consumer.Pending
	}
	return total
}
