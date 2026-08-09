-- Canonical AT Protocol identity, OAuth custody, provisioning audit, and
-- projection state. Secret-bearing OAuth values are encrypted by the API
-- before they reach PostgreSQL.

ALTER TABLE profiles ADD COLUMN created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE places ADD COLUMN created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE tours ADD COLUMN created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE appearances ADD COLUMN created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE places ADD COLUMN publication_status TEXT NOT NULL DEFAULT 'draft'
    CHECK (publication_status IN ('draft', 'published', 'archived'));
ALTER TABLE venues ADD COLUMN publication_status TEXT NOT NULL DEFAULT 'draft'
    CHECK (publication_status IN ('draft', 'published', 'archived'));
ALTER TABLE acts ADD COLUMN publication_status TEXT NOT NULL DEFAULT 'draft'
    CHECK (publication_status IN ('draft', 'published', 'archived'));
ALTER TABLE tours ADD COLUMN publication_status TEXT NOT NULL DEFAULT 'draft'
    CHECK (publication_status IN ('draft', 'published', 'archived'));
ALTER TABLE appearances ADD COLUMN publication_status TEXT NOT NULL DEFAULT 'draft'
    CHECK (publication_status IN ('draft', 'published', 'archived'));

CREATE TABLE atproto_oauth_links (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    account_did TEXT NOT NULL UNIQUE CHECK (account_did LIKE 'did:%'),
    handle TEXT,
    session_id TEXT NOT NULL,
    host_url TEXT NOT NULL CHECK (host_url LIKE 'https://%'),
    granted_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'expired', 'revoked', 'error')),
    linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE TABLE atproto_oauth_sessions (
    account_did TEXT NOT NULL,
    session_id TEXT NOT NULL,
    encrypted_session BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_did, session_id)
);

CREATE TABLE atproto_oauth_requests (
    state_hash CHAR(64) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_request BYTEA NOT NULL,
    return_path TEXT NOT NULL DEFAULT '/settings',
    requested_scope TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (return_path LIKE '/%')
);
CREATE INDEX atproto_oauth_requests_pending
    ON atproto_oauth_requests(expires_at) WHERE consumed_at IS NULL;

CREATE TABLE atproto_provisioning_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    handle TEXT NOT NULL,
    invitation_hash CHAR(64),
    status TEXT NOT NULL
        CHECK (status IN ('requested', 'issued', 'consumed', 'expired', 'rejected')),
    terms_version TEXT NOT NULL,
    turnstile_outcome TEXT NOT NULL
        CHECK (turnstile_outcome IN ('passed', 'failed', 'unavailable')),
    ip_hash CHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX atproto_provisioning_one_active_user
    ON atproto_provisioning_requests(user_id)
    WHERE status IN ('requested', 'issued');
CREATE UNIQUE INDEX atproto_provisioning_one_active_handle
    ON atproto_provisioning_requests(handle)
    WHERE status IN ('requested', 'issued', 'consumed');
CREATE INDEX atproto_provisioning_daily_user
    ON atproto_provisioning_requests(user_id, created_at DESC);
CREATE INDEX atproto_provisioning_daily_ip
    ON atproto_provisioning_requests(ip_hash, created_at DESC);

CREATE TABLE atproto_record_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    publisher_did TEXT NOT NULL CHECK (publisher_did LIKE 'did:%'),
    collection TEXT NOT NULL CHECK (collection LIKE 'tv.subcult.%'),
    rkey TEXT NOT NULL,
    at_uri TEXT NOT NULL UNIQUE CHECK (at_uri LIKE 'at://%'),
    cid TEXT,
    projection_status TEXT NOT NULL DEFAULT 'reserved'
        CHECK (projection_status IN (
            'reserved', 'awaiting_projection', 'projected', 'failed',
            'conflict', 'deleted', 'quarantined'
        )),
    record_version BIGINT NOT NULL DEFAULT 1,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (publisher_did, collection, rkey),
    UNIQUE (entity_type, entity_id)
);
CREATE INDEX atproto_record_projection_queue
    ON atproto_record_mappings(projection_status, updated_at)
    WHERE projection_status = 'awaiting_projection';

CREATE TABLE atproto_projection_checkpoints (
    source TEXT PRIMARY KEY,
    cursor TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB
);

CREATE TABLE atproto_projection_failures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source TEXT NOT NULL,
    publisher_did TEXT,
    collection TEXT,
    rkey TEXT,
    cid TEXT,
    reason_code TEXT NOT NULL,
    safe_detail TEXT,
    payload_digest CHAR(64),
    quarantined BOOLEAN NOT NULL DEFAULT TRUE,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX atproto_projection_failures_open
    ON atproto_projection_failures(last_seen_at DESC)
    WHERE resolved_at IS NULL;

INSERT INTO schema_version(version, description)
SELECT 41, 'canonical AT Protocol OAuth, provisioning, and projection state'
WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 41);
