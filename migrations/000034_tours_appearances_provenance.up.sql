-- Add touring relations while preserving events.scene_id as the legacy primary
-- scene relation. event_hosts adds multi-host context; it does not replace it.

ALTER TABLE events ADD COLUMN IF NOT EXISTS place_id UUID REFERENCES places(id) ON DELETE RESTRICT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS venue_id UUID REFERENCES venues(id) ON DELETE RESTRICT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'show'
    CHECK (kind IN ('show', 'festival', 'party', 'meetup', 'broadcast', 'other'));

CREATE INDEX IF NOT EXISTS idx_events_place_time
    ON events(place_id, starts_at)
    WHERE deleted_at IS NULL AND cancelled_at IS NULL AND place_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_events_venue_time
    ON events(venue_id, starts_at)
    WHERE deleted_at IS NULL AND cancelled_at IS NULL AND venue_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_events_kind_time
    ON events(kind, starts_at)
    WHERE deleted_at IS NULL AND cancelled_at IS NULL;

CREATE TABLE event_hosts (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    scene_id UUID REFERENCES scenes(id) ON DELETE CASCADE,
    profile_id UUID REFERENCES profiles(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('host', 'promoter', 'venue', 'publisher')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((scene_id IS NOT NULL)::int + (profile_id IS NOT NULL)::int = 1)
);

CREATE UNIQUE INDEX event_hosts_scene_unique
    ON event_hosts (event_id, scene_id, role) WHERE scene_id IS NOT NULL;
CREATE UNIQUE INDEX event_hosts_profile_unique
    ON event_hosts (event_id, profile_id, role) WHERE profile_id IS NOT NULL;
CREATE INDEX idx_event_hosts_scene ON event_hosts(scene_id) WHERE scene_id IS NOT NULL;
CREATE INDEX idx_event_hosts_profile ON event_hosts(profile_id) WHERE profile_id IS NOT NULL;

-- Every existing Event retains its original scene as a compatibility host.
INSERT INTO event_hosts (event_id, scene_id, role)
SELECT id, scene_id, 'host' FROM events
ON CONFLICT DO NOTHING;

CREATE TABLE tours (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    primary_act_id UUID NOT NULL REFERENCES acts(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    status TEXT NOT NULL CHECK (status IN ('draft', 'announced', 'changed', 'cancelled', 'completed')),
    starts_on DATE,
    ends_on DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_on IS NULL OR starts_on IS NULL OR ends_on >= starts_on)
);

CREATE INDEX idx_tours_primary_act_status ON tours(primary_act_id, status);

CREATE TABLE tour_acts (
    tour_id UUID NOT NULL REFERENCES tours(id) ON DELETE CASCADE,
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('primary', 'co_headliner', 'support', 'guest')),
    added_by_did TEXT NOT NULL CHECK (btrim(added_by_did) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tour_id, act_id, role)
);

CREATE INDEX idx_tour_acts_act ON tour_acts(act_id);

CREATE TABLE appearances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    tour_id UUID REFERENCES tours(id) ON DELETE SET NULL,
    role TEXT NOT NULL CHECK (role IN ('headliner', 'support', 'performer', 'dj', 'speaker', 'host', 'other')),
    stage_name TEXT,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    status TEXT NOT NULL CHECK (status IN ('announced', 'confirmed', 'cancelled', 'completed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at),
    UNIQUE NULLS NOT DISTINCT (event_id, act_id, role, starts_at)
);

CREATE INDEX idx_appearances_event ON appearances(event_id);
CREATE INDEX idx_appearances_act_time ON appearances(act_id, starts_at);
CREATE INDEX idx_appearances_tour_time ON appearances(tour_id, starts_at) WHERE tour_id IS NOT NULL;

CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider TEXT NOT NULL CHECK (btrim(provider) <> ''),
    external_id TEXT,
    canonical_url TEXT,
    payload_sha256 CHAR(64) NOT NULL CHECK (payload_sha256 ~ '^[0-9A-Fa-f]{64}$'),
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (external_id IS NOT NULL OR canonical_url IS NOT NULL),
    CHECK (last_seen_at >= first_seen_at),
    UNIQUE NULLS NOT DISTINCT (provider, external_id, canonical_url)
);

CREATE INDEX idx_sources_provider_external_id ON sources(provider, external_id) WHERE external_id IS NOT NULL;

CREATE TABLE entity_assertions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('event', 'appearance', 'tour', 'profile', 'venue')),
    entity_id UUID NOT NULL,
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,
    state TEXT NOT NULL CHECK (state IN ('unverified', 'claimed', 'verified', 'disputed', 'rejected')),
    submitter_type TEXT NOT NULL CHECK (submitter_type IN ('did', 'integration')),
    submitted_by_did TEXT,
    integration_id TEXT,
    authority_type TEXT NOT NULL CHECK (authority_type IN ('artist', 'host', 'venue', 'promoter', 'ticketing_provider', 'community_proposal')),
    asserted_fields JSONB NOT NULL CHECK (jsonb_typeof(asserted_fields) = 'object' AND asserted_fields <> '{}'::jsonb),
    rationale TEXT,
    reviewed_by_did TEXT,
    reviewed_at TIMESTAMPTZ,
    asserted_at TIMESTAMPTZ NOT NULL,
    supersedes_id UUID REFERENCES entity_assertions(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (submitter_type = 'did' AND submitted_by_did IS NOT NULL AND integration_id IS NULL)
        OR (submitter_type = 'integration' AND integration_id IS NOT NULL AND submitted_by_did IS NULL)
    ),
    CHECK ((reviewed_by_did IS NULL) = (reviewed_at IS NULL)),
    CHECK (supersedes_id IS NULL OR supersedes_id <> id)
);

CREATE INDEX idx_entity_assertions_entity ON entity_assertions(entity_type, entity_id, asserted_at DESC);
CREATE INDEX idx_entity_assertions_source ON entity_assertions(source_id);
CREATE INDEX idx_entity_assertions_supersedes ON entity_assertions(supersedes_id) WHERE supersedes_id IS NOT NULL;

COMMENT ON TABLE event_hosts IS 'Additional event host context; events.scene_id remains the compatible primary scene relation.';
COMMENT ON TABLE tours IS 'Act-led itinerary grouping appearances without replacing Event occurrences.';
COMMENT ON TABLE appearances IS 'Act participation in an Event; tour-stop, festival, and one-off labels are projections.';
COMMENT ON TABLE sources IS 'Observed source identity and payload digest, not an unbounded raw provider payload store.';
COMMENT ON TABLE entity_assertions IS 'Attributed claims with authority, review state, asserted fields, and correction lineage.';
