-- Durable publication state for dead-letter events.
ALTER TABLE dead_letter_events
    ADD COLUMN IF NOT EXISTS publish_attempts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE dead_letter_events
    ADD COLUMN IF NOT EXISTS last_publish_error TEXT;

ALTER TABLE dead_letter_events
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_dead_letter_events_unpublished
    ON dead_letter_events(published_at, created_at);
