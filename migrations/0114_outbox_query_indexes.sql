-- +goose Up
CREATE INDEX idx_outbox_status_topic_partition_created
  ON outbox (status, topic, partition_key, created_at DESC, id DESC);

CREATE INDEX idx_outbox_status_topic_created
  ON outbox (status, topic, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_status_topic_partition_created;
DROP INDEX IF EXISTS idx_outbox_status_topic_created;
