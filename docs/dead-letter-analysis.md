# Dead Letter Analysis

## Current Design

死信链路分两层：

- Redis Stream dead-letter：短期缓冲队列，用于把 poison message 从主消费组移走。
- PostgreSQL `dead_letter_events`：长期分析表，用于统一查询、聚合、外部系统拉取和后续人工/自动分析。

当前进入死信的来源：

- Redis Stream message 在 consumer group 中 pending 后被 reclaim，投递次数超过 3 次。
- PostgreSQL local outbox `async_messages` 派发 Redis 失败累计到第 3 次。

当前处理流程：

```text
main stream
  -> worker consumer group
  -> pending reclaim
  -> delivery count >= 3
  -> Redis dead-letter stream
  -> dead-letter analyzer consumer
  -> PostgreSQL dead_letter_events
  -> /api/ops/dead-letters*
```

Outbox 派发失败不依赖 Redis dead-letter stream，会直接写入 `dead_letter_events` 并把 `async_messages.status` 标记为 `dead_letter`。

## External API

这些 API 只允许 root 调用：

```text
GET /api/ops/dead-letters/summary
GET /api/ops/dead-letters?status=new&source=redis_stream&limit=100
GET /api/ops/dead-letters/{dead_letter_id}
```

外部系统应该读取 PostgreSQL 标准化后的 API，不要直接依赖 Redis stream 内部格式。

## Optimization Space

当前实现够 MVP 使用，但还有优化空间：

- 阈值配置化：现在失败 3 次进入死信是固定规则，后续可按 event_type 配置不同阈值。
- 错误分类：现在保存 reason/error_text，后续可增加 error_class、retryable、owner、severity。
- 重放能力：后续可以增加 root-only replay API，把修复后的死信重新投递回主 stream。
- 告警聚合：按 event_type、aggregate_type、error_class 聚合，超过阈值触发通知。
- 保留策略：Redis dead-letter stream 和 `dead_letter_events` 都需要 TTL/归档策略。
- 分析状态流转：目前状态有 new/analyzing/resolved/ignored，后续可增加处理人、备注、解决时间。
- 幂等键增强：当前按 source/source_stream/source_message_id 去重；跨源同业务错误可再增加 business_key。
