-- Jetstream v2 uses a monotonic uint64 sequence which is unrelated to the
-- legacy Jetstream v1 time_us cursor in indexer_state. Keep both namespaces
-- physically separate so an old timestamp can never be interpreted as a v2
-- sequence.

CREATE TABLE jetstream_v2_cursors (
    consumer TEXT NOT NULL,
    target TEXT NOT NULL CHECK (target IN ('active', 'shadow')),
    rebuild_id TEXT NOT NULL DEFAULT '',
    cursor NUMERIC(20, 0) NOT NULL DEFAULT 0 CHECK (cursor >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (consumer, target, rebuild_id)
);

CREATE TABLE jetstream_v2_accounts (
    did TEXT PRIMARY KEY CHECK (did LIKE 'did:%'),
    active BOOLEAN NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    relay_seq BIGINT,
    event_time TIMESTAMPTZ,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE jetstream_v2_identities (
    did TEXT PRIMARY KEY CHECK (did LIKE 'did:%'),
    handle TEXT NOT NULL DEFAULT '',
    relay_seq BIGINT,
    event_time TIMESTAMPTZ,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE jetstream_v2_reconciliations (
    did TEXT PRIMARY KEY CHECK (did LIKE 'did:%'),
    requested_rev TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL CHECK (reason IN ('sync', 'account_reactivated')),
    relay_seq BIGINT,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX jetstream_v2_reconciliations_pending
    ON jetstream_v2_reconciliations(requested_at, did)
    WHERE status = 'pending';

-- A rebuild folds archive events into an isolated AT-URI keyed projection.
-- It is intentionally raw: promotion into product tables is a separate,
-- reviewable operation after comparison and data-quality checks.
CREATE TABLE jetstream_v2_shadow_records (
    rebuild_id TEXT NOT NULL,
    at_uri TEXT NOT NULL CHECK (at_uri LIKE 'at://%'),
    did TEXT NOT NULL CHECK (did LIKE 'did:%'),
    collection TEXT NOT NULL,
    rkey TEXT NOT NULL,
    rev TEXT NOT NULL DEFAULT '',
    cid TEXT NOT NULL DEFAULT '',
    record JSONB,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    suppressed BOOLEAN NOT NULL DEFAULT FALSE,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    time_us BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (rebuild_id, at_uri)
);
CREATE INDEX jetstream_v2_shadow_records_analysis
    ON jetstream_v2_shadow_records(rebuild_id, collection, time_us)
    WHERE deleted = FALSE AND suppressed = FALSE;
CREATE INDEX jetstream_v2_shadow_records_did
    ON jetstream_v2_shadow_records(rebuild_id, did)
    WHERE deleted = FALSE AND suppressed = FALSE;

CREATE TABLE jetstream_v2_shadow_accounts (
    rebuild_id TEXT NOT NULL,
    did TEXT NOT NULL CHECK (did LIKE 'did:%'),
    active BOOLEAN NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    relay_seq BIGINT,
    event_time TIMESTAMPTZ,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (rebuild_id, did)
);

CREATE TABLE jetstream_v2_shadow_identities (
    rebuild_id TEXT NOT NULL,
    did TEXT NOT NULL CHECK (did LIKE 'did:%'),
    handle TEXT NOT NULL DEFAULT '',
    relay_seq BIGINT,
    event_time TIMESTAMPTZ,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (rebuild_id, did)
);

CREATE TABLE jetstream_v2_shadow_reconciliations (
    rebuild_id TEXT NOT NULL,
    did TEXT NOT NULL CHECK (did LIKE 'did:%'),
    requested_rev TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL CHECK (reason IN ('sync', 'account_reactivated')),
    relay_seq BIGINT,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (rebuild_id, did)
);

CREATE TABLE jetstream_v2_shadow_failures (
    rebuild_id TEXT NOT NULL,
    jetstream_seq NUMERIC(20, 0) NOT NULL CHECK (jetstream_seq >= 0),
    did TEXT NOT NULL CHECK (did LIKE 'did:%'),
    collection TEXT NOT NULL,
    rkey TEXT NOT NULL,
    reason_code TEXT NOT NULL,
    safe_detail TEXT,
    payload_digest CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (rebuild_id, jetstream_seq)
);

ALTER TABLE backfill_checkpoints
    ADD COLUMN cursor_seq NUMERIC(20, 0) NOT NULL DEFAULT 0 CHECK (cursor_seq >= 0),
    ADD COLUMN target TEXT NOT NULL DEFAULT 'active'
        CHECK (target IN ('active', 'shadow')),
    ADD COLUMN rebuild_id TEXT NOT NULL DEFAULT '';

-- Counts and one-sided differences provide a cheap pre-release data-quality
-- gate. Detailed rows remain queryable by AT URI in the shadow table.
CREATE VIEW jetstream_v2_projection_comparison AS
WITH current_records AS (
    SELECT 'at://' || record_did || '/app.subcult.scene/' || record_rkey AS at_uri
      FROM scenes WHERE record_did IS NOT NULL AND record_rkey IS NOT NULL AND deleted_at IS NULL
    UNION ALL
    SELECT 'at://' || record_did || '/app.subcult.event/' || record_rkey
      FROM events WHERE record_did IS NOT NULL AND record_rkey IS NOT NULL AND deleted_at IS NULL
    UNION ALL
    SELECT 'at://' || record_did || '/app.subcult.post/' || record_rkey
      FROM posts WHERE record_did IS NOT NULL AND record_rkey IS NOT NULL AND deleted_at IS NULL
    UNION ALL
    SELECT 'at://' || record_did || '/app.subcult.alliance/' || record_rkey
      FROM alliances WHERE record_did IS NOT NULL AND record_rkey IS NOT NULL AND deleted_at IS NULL
    UNION ALL
    SELECT at_uri FROM atproto_record_mappings WHERE projection_status = 'projected'
), rebuilds AS (
    SELECT DISTINCT rebuild_id FROM jetstream_v2_shadow_records
)
SELECT r.rebuild_id,
       (SELECT COUNT(*) FROM current_records) AS current_count,
       (SELECT COUNT(*) FROM jetstream_v2_shadow_records s
         WHERE s.rebuild_id = r.rebuild_id AND s.deleted = FALSE AND s.suppressed = FALSE) AS shadow_count,
       (SELECT COUNT(*) FROM current_records c
         WHERE NOT EXISTS (
             SELECT 1 FROM jetstream_v2_shadow_records s
              WHERE s.rebuild_id = r.rebuild_id AND s.at_uri = c.at_uri
                AND s.deleted = FALSE AND s.suppressed = FALSE
         )) AS only_current_count,
       (SELECT COUNT(*) FROM jetstream_v2_shadow_records s
         WHERE s.rebuild_id = r.rebuild_id AND s.deleted = FALSE AND s.suppressed = FALSE
           AND NOT EXISTS (SELECT 1 FROM current_records c WHERE c.at_uri = s.at_uri)) AS only_shadow_count
  FROM rebuilds r;

INSERT INTO schema_version(version, description)
SELECT 46, 'Jetstream v2 sequence cursors, event state, reconciliation, and shadow rebuilds'
WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 46);
