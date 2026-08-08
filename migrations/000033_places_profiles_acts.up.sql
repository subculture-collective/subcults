-- Add normalized touring identities and geographic context.
-- Event occurrence locations remain on events until migration 000034 links them.

CREATE TABLE places (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    canonical_name TEXT NOT NULL CHECK (btrim(canonical_name) <> ''),
    admin_region TEXT,
    country_code CHAR(2) NOT NULL CHECK (country_code = upper(country_code)),
    timezone TEXT NOT NULL CHECK (btrim(timezone) <> ''),
    coarse_geohash VARCHAR(20) NOT NULL CHECK (btrim(coarse_geohash) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (canonical_name, admin_region, country_code)
);

CREATE INDEX idx_places_coarse_geohash ON places(coarse_geohash);

CREATE TABLE venues (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    place_id UUID NOT NULL REFERENCES places(id) ON DELETE RESTRICT,
    canonical_name TEXT NOT NULL CHECK (btrim(canonical_name) <> ''),
    allow_precise BOOLEAN NOT NULL DEFAULT FALSE,
    precise_point GEOGRAPHY(Point, 4326),
    coarse_geohash VARCHAR(20) NOT NULL CHECK (btrim(coarse_geohash) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_venue_precise_consent CHECK (allow_precise OR precise_point IS NULL),
    UNIQUE (place_id, canonical_name)
);

CREATE INDEX idx_venues_place ON venues(place_id);
CREATE INDEX idx_venues_coarse_geohash ON venues(coarse_geohash);
CREATE INDEX idx_venues_public_location ON venues USING GIST(precise_point)
    WHERE precise_point IS NOT NULL AND allow_precise = TRUE;

CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    kind TEXT NOT NULL CHECK (kind IN (
        'artist', 'venue', 'festival', 'promoter', 'collective', 'label', 'curator'
    )),
    canonical_name TEXT NOT NULL CHECK (btrim(canonical_name) <> ''),
    visibility TEXT NOT NULL DEFAULT 'public'
        CHECK (visibility IN ('public', 'private', 'unlisted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_profiles_kind_visibility ON profiles(kind, visibility);

CREATE TABLE profile_controllers (
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    controller_did TEXT NOT NULL CHECK (btrim(controller_did) <> ''),
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (profile_id, controller_did)
);

CREATE TABLE acts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID NOT NULL UNIQUE REFERENCES profiles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE act_scene_affiliations (
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    scene_id UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    relationship TEXT NOT NULL CHECK (relationship IN ('home', 'member', 'associated')),
    asserted_by_did TEXT NOT NULL CHECK (btrim(asserted_by_did) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (act_id, scene_id, relationship)
);

CREATE INDEX idx_act_scene_affiliations_scene ON act_scene_affiliations(scene_id);

CREATE TABLE act_home_territories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    act_id UUID NOT NULL REFERENCES acts(id) ON DELETE CASCADE,
    place_id UUID NOT NULL REFERENCES places(id) ON DELETE RESTRICT,
    visibility TEXT NOT NULL CHECK (visibility IN ('public', 'private', 'unlisted')),
    valid_from DATE NOT NULL,
    valid_to DATE,
    asserted_by_did TEXT NOT NULL CHECK (btrim(asserted_by_did) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_home_territory_dates CHECK (valid_to IS NULL OR valid_to >= valid_from),
    UNIQUE NULLS NOT DISTINCT (act_id, place_id, valid_from, valid_to)
);

CREATE INDEX idx_act_home_territories_current
    ON act_home_territories(act_id, valid_from, valid_to);

COMMENT ON TABLE places IS 'Canonical city, market, or regional context used for discovery.';
COMMENT ON TABLE venues IS 'Named event location with independent precise-location retention consent.';
COMMENT ON TABLE profiles IS 'Public-facing identity controlled by one or more DIDs.';
COMMENT ON TABLE acts IS 'Creative project that may appear at events.';
COMMENT ON TABLE act_home_territories IS 'Declared coarse Act-to-Place affinity; never a residence or live location.';
