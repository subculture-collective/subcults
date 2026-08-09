-- Tap outbox identifiers are unsigned machine-sized values. BIGINT prevents
-- durable synchronization from failing once the source exceeds int32 range.
ALTER TABLE atproto_sync_observations
    ALTER COLUMN source_event_id TYPE BIGINT;
