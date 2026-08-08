-- Public beta foundation: passwordless identity, creator approval, stable
-- publication identifiers, optimistic concurrency, and protected locations.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS internal_did TEXT,
    ADD COLUMN IF NOT EXISTS display_name TEXT,
    ADD COLUMN IF NOT EXISTS onboarding_complete BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE users
SET internal_did = COALESCE(internal_did, did, 'did:web:subcults.subcult.tv:users:' || id::text)
WHERE internal_did IS NULL;

ALTER TABLE users ALTER COLUMN internal_did SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS users_internal_did_unique ON users(internal_did);

CREATE TABLE auth_email_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    encrypted_email BYTEA NOT NULL,
    email_hmac CHAR(64) NOT NULL UNIQUE,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE auth_magic_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_identity_id UUID NOT NULL REFERENCES auth_email_identities(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL UNIQUE,
    return_path TEXT NOT NULL DEFAULT '/',
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (return_path LIKE '/%'),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);
CREATE INDEX auth_magic_links_pending
    ON auth_magic_links(token_hash, expires_at) WHERE consumed_at IS NULL;

CREATE TABLE auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash CHAR(64) NOT NULL UNIQUE,
    user_agent_hash CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    idle_expires_at TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (idle_expires_at <= absolute_expires_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE INDEX auth_sessions_active_user
    ON auth_sessions(user_id, idle_expires_at) WHERE revoked_at IS NULL;

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('participant', 'creator_pending', 'creator', 'admin')),
    granted_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, role, granted_at),
    CHECK (revoked_at IS NULL OR revoked_at >= granted_at)
);
CREATE UNIQUE INDEX user_roles_one_active_role
    ON user_roles(user_id) WHERE revoked_at IS NULL;

INSERT INTO user_roles (user_id, role)
SELECT id, 'participant' FROM users
ON CONFLICT DO NOTHING;

CREATE TABLE creator_access_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    statement TEXT NOT NULL CHECK (length(btrim(statement)) BETWEEN 20 AND 2000),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn')),
    reviewed_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    review_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((reviewed_by_user_id IS NULL) = (reviewed_at IS NULL))
);
CREATE UNIQUE INDEX creator_access_one_pending
    ON creator_access_requests(user_id) WHERE status = 'pending';

ALTER TABLE scenes
    ADD COLUMN IF NOT EXISTS public_slug TEXT,
    ADD COLUMN IF NOT EXISTS publication_status TEXT NOT NULL DEFAULT 'published'
        CHECK (publication_status IN ('draft', 'published', 'archived')),
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
CREATE UNIQUE INDEX IF NOT EXISTS scenes_public_slug_unique
    ON scenes(public_slug) WHERE public_slug IS NOT NULL;

ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS public_slug TEXT,
    ADD COLUMN IF NOT EXISTS publication_status TEXT NOT NULL DEFAULT 'published'
        CHECK (publication_status IN ('draft', 'published', 'archived')),
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
CREATE UNIQUE INDEX IF NOT EXISTS profiles_public_slug_unique
    ON profiles(public_slug) WHERE public_slug IS NOT NULL;

ALTER TABLE tours
    ADD COLUMN IF NOT EXISTS public_slug TEXT,
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
CREATE UNIQUE INDEX IF NOT EXISTS tours_public_slug_unique
    ON tours(public_slug) WHERE public_slug IS NOT NULL;

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS public_slug TEXT,
    ADD COLUMN IF NOT EXISTS publication_status TEXT NOT NULL DEFAULT 'published'
        CHECK (publication_status IN ('draft', 'published', 'archived')),
    ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS postponed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS location_access TEXT NOT NULL DEFAULT 'public'
        CHECK (location_access IN ('public', 'protected'));
CREATE UNIQUE INDEX IF NOT EXISTS events_public_slug_unique
    ON events(public_slug) WHERE public_slug IS NOT NULL;

CREATE TABLE event_location_grants (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT NOT NULL CHECK (reason IN ('membership', 'rsvp', 'ticket', 'manual')),
    granted_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (event_id, user_id, reason, granted_at),
    CHECK (expires_at IS NULL OR expires_at > granted_at),
    CHECK (revoked_at IS NULL OR revoked_at >= granted_at)
);
CREATE INDEX event_location_grants_active
    ON event_location_grants(event_id, user_id)
    WHERE revoked_at IS NULL;

COMMENT ON TABLE auth_magic_links IS 'One-time, enumeration-resistant passwordless authentication challenges.';
COMMENT ON TABLE auth_sessions IS 'Rotating refresh sessions; raw refresh tokens are never persisted.';
COMMENT ON TABLE creator_access_requests IS 'Approval boundary for public Studio publishing during beta.';
COMMENT ON TABLE event_location_grants IS 'Explicit authorization for protected event occurrence details.';
