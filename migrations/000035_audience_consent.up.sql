-- Private audience, contact-link, consent, and suppression ledger.
-- This migration depends on 000033 (places/profiles) and 000034
-- (tours/appearances). It intentionally contains no delivery-provider state.

CREATE TABLE contact_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('email', 'phone', 'web_push', 'apns', 'provider_social')),
    encrypted_value BYTEA NOT NULL,
    value_hmac CHAR(64) NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (kind, value_hmac)
);

CREATE TABLE contact_point_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id) ON DELETE CASCADE,
    user_did TEXT NOT NULL,
    verification_method TEXT NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    verified_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (revoked_at IS NULL OR revoked_at >= verified_at)
);
CREATE INDEX idx_contact_point_links_active_did
    ON contact_point_links (user_did) WHERE revoked_at IS NULL;

CREATE TABLE audience_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_did TEXT,
    contact_point_id UUID REFERENCES contact_points(id) ON DELETE CASCADE,
    program_type TEXT NOT NULL CHECK (program_type IN ('scene', 'profile')),
    program_id UUID NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('membership', 'rsvp', 'attendance', 'purchase', 'stream', 'interest')),
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    CHECK ((subject_did IS NOT NULL)::int + (contact_point_id IS NOT NULL)::int = 1)
);
CREATE INDEX idx_audience_relationships_program
    ON audience_relationships (program_type, program_id, kind, occurred_at DESC);

CREATE TABLE consent_scopes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_type TEXT NOT NULL CHECK (sender_type IN ('scene', 'profile')),
    sender_id UUID NOT NULL,
    channel TEXT NOT NULL,
    purpose TEXT NOT NULL,
    tour_id UUID REFERENCES tours(id) ON DELETE CASCADE,
    event_id UUID REFERENCES events(id) ON DELETE CASCADE,
    appearance_id UUID REFERENCES appearances(id) ON DELETE CASCADE,
    place_id UUID REFERENCES places(id),
    disclosure_version TEXT NOT NULL,
    region TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (
        sender_type, sender_id, channel, purpose,
        tour_id, event_id, appearance_id, place_id, disclosure_version, region
    )
);
CREATE INDEX idx_consent_scopes_sender
    ON consent_scopes (sender_type, sender_id, channel, purpose);

CREATE TABLE consent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id) ON DELETE CASCADE,
    consent_scope_id UUID NOT NULL REFERENCES consent_scopes(id) ON DELETE RESTRICT,
    action TEXT NOT NULL CHECK (action IN ('grant', 'revoke')),
    capture_source TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX idx_consent_events_contact_scope_time
    ON consent_events (contact_point_id, consent_scope_id, occurred_at DESC, id DESC);

CREATE TABLE suppressions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id) ON DELETE CASCADE,
    level TEXT NOT NULL CHECK (level IN ('global', 'channel', 'sender', 'scope')),
    channel TEXT,
    sender_type TEXT CHECK (sender_type IN ('scene', 'profile')),
    sender_id UUID,
    consent_scope_id UUID REFERENCES consent_scopes(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    lifted_at TIMESTAMPTZ,
    CHECK (lifted_at IS NULL OR lifted_at >= occurred_at),
    CHECK (
        (level = 'global' AND channel IS NULL AND sender_type IS NULL AND sender_id IS NULL AND consent_scope_id IS NULL)
        OR (level = 'channel' AND channel IS NOT NULL AND sender_type IS NULL AND sender_id IS NULL AND consent_scope_id IS NULL)
        OR (level = 'sender' AND sender_type IS NOT NULL AND sender_id IS NOT NULL AND consent_scope_id IS NULL)
        OR (level = 'scope' AND channel IS NULL AND sender_type IS NULL AND sender_id IS NULL AND consent_scope_id IS NOT NULL)
    )
);
CREATE INDEX idx_suppressions_contact_active
    ON suppressions (contact_point_id, level, occurred_at DESC) WHERE lifted_at IS NULL;
