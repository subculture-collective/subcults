CREATE TABLE signals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type TEXT NOT NULL CHECK (owner_type IN ('scene','profile')),
    owner_id UUID NOT NULL,
    target_type TEXT NOT NULL CHECK (target_type IN ('scene','profile','event','appearance','tour','post','stream','offer')),
    target_id UUID NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('draft','scheduled','published','completed','cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE signal_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signal_id UUID NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    revision INT NOT NULL,
    content JSONB NOT NULL,
    audience_definition JSONB NOT NULL,
    publish_at TIMESTAMPTZ,
    created_by_did TEXT NOT NULL,
    supersedes_signal_revision_id UUID REFERENCES signal_revisions(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (signal_id, revision)
);

CREATE TABLE signal_consent_scopes (
    signal_id UUID NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    consent_scope_id UUID NOT NULL REFERENCES consent_scopes(id) ON DELETE RESTRICT,
    PRIMARY KEY (signal_id, consent_scope_id)
);

CREATE TABLE deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    signal_revision_id UUID NOT NULL REFERENCES signal_revisions(id),
    contact_point_id UUID NOT NULL REFERENCES contact_points(id),
    channel TEXT NOT NULL CHECK (channel IN ('web_push','email')),
    purpose TEXT NOT NULL,
    provider TEXT NOT NULL,
    authorization_scope JSONB NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued','suppressed','sent','delivered','failed','cancelled')),
    provider_message_id TEXT,
    scheduled_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (signal_revision_id, contact_point_id, channel)
);

CREATE TABLE engagement_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id UUID REFERENCES deliveries(id),
    signal_id UUID NOT NULL REFERENCES signals(id),
    kind TEXT NOT NULL CHECK (kind IN ('view','click','rsvp','purchase','unsubscribe','complaint')),
    event_id UUID REFERENCES events(id),
    appearance_id UUID REFERENCES appearances(id),
    tour_id UUID REFERENCES tours(id),
    occurred_at TIMESTAMPTZ NOT NULL,
    provenance JSONB NOT NULL
);
