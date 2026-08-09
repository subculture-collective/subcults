CREATE TABLE atproto_sync_observations (
    id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL CHECK (source IN ('tap', 'jetstream', 'reconciliation')),
    source_event_id BIGINT,
    publisher_did TEXT NOT NULL CHECK (publisher_did LIKE 'did:%'),
    collection TEXT NOT NULL,
    rkey TEXT NOT NULL,
    at_uri TEXT NOT NULL CHECK (at_uri LIKE 'at://%'),
    cid TEXT,
    revision TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('create', 'update', 'delete')),
    payload_digest CHAR(64),
    projection_outcome TEXT NOT NULL CHECK (projection_outcome IN ('projected', 'deleted', 'legacy_observed', 'quarantined')),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (source, source_event_id),
    UNIQUE (source, publisher_did, collection, rkey, revision, action)
);
CREATE INDEX atproto_sync_observations_record_history
    ON atproto_sync_observations(publisher_did, collection, rkey, observed_at DESC);

INSERT INTO schema_version(version, description)
SELECT 42, 'durable AT Protocol sync observations and correction history'
WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 42);
