-- Durable beta guarantees: align RSVP identity with DID-backed application
-- identities, make every Studio-authored aggregate versioned, and enforce
-- immutable Signal revisions in the database itself.

ALTER TABLE event_rsvps
    ALTER COLUMN user_id TYPE TEXT USING user_id::text;

ALTER TABLE places ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE appearances ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;

-- Domain revision and delivery identifiers are stable compound identifiers
-- (for example <signal>-r2), not UUIDs. Keep the public IDs lossless.
ALTER TABLE signal_revisions DROP CONSTRAINT IF EXISTS signal_revisions_supersedes_signal_revision_id_fkey;
ALTER TABLE deliveries DROP CONSTRAINT IF EXISTS deliveries_signal_revision_id_fkey;
ALTER TABLE engagement_events DROP CONSTRAINT IF EXISTS engagement_events_delivery_id_fkey;
ALTER TABLE payment_records DROP CONSTRAINT IF EXISTS payment_records_delivery_id_fkey;
ALTER TABLE signal_revisions ALTER COLUMN id TYPE TEXT USING id::text;
ALTER TABLE signal_revisions ALTER COLUMN supersedes_signal_revision_id TYPE TEXT USING supersedes_signal_revision_id::text;
ALTER TABLE deliveries ALTER COLUMN id TYPE TEXT USING id::text;
ALTER TABLE deliveries ALTER COLUMN signal_revision_id TYPE TEXT USING signal_revision_id::text;
ALTER TABLE engagement_events ALTER COLUMN delivery_id TYPE TEXT USING delivery_id::text;
ALTER TABLE payment_records ALTER COLUMN delivery_id TYPE TEXT USING delivery_id::text;
ALTER TABLE signal_revisions ADD CONSTRAINT signal_revisions_supersedes_signal_revision_id_fkey
    FOREIGN KEY (supersedes_signal_revision_id) REFERENCES signal_revisions(id);
ALTER TABLE deliveries ADD CONSTRAINT deliveries_signal_revision_id_fkey
    FOREIGN KEY (signal_revision_id) REFERENCES signal_revisions(id);
ALTER TABLE engagement_events ADD CONSTRAINT engagement_events_delivery_id_fkey
    FOREIGN KEY (delivery_id) REFERENCES deliveries(id);
ALTER TABLE payment_records ADD CONSTRAINT payment_records_delivery_id_fkey
    FOREIGN KEY (delivery_id) REFERENCES deliveries(id) ON DELETE SET NULL;

CREATE OR REPLACE FUNCTION reject_signal_revision_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'signal revisions are immutable'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER signal_revisions_immutable_update
    BEFORE UPDATE ON signal_revisions
    FOR EACH ROW EXECUTE FUNCTION reject_signal_revision_mutation();

CREATE TRIGGER signal_revisions_immutable_delete
    BEFORE DELETE ON signal_revisions
    FOR EACH ROW EXECUTE FUNCTION reject_signal_revision_mutation();

-- A correction can supersede one assertion once. The prior assertion remains
-- present and queryable, preserving the complete provenance chain.
CREATE UNIQUE INDEX entity_assertions_one_correction
    ON entity_assertions(supersedes_id)
    WHERE supersedes_id IS NOT NULL;

CREATE INDEX consent_events_effective_state
    ON consent_events(contact_point_id, consent_scope_id, occurred_at DESC, id DESC)
    INCLUDE (action);

CREATE INDEX deliveries_dispatch_queue
    ON deliveries(state, scheduled_at)
    WHERE state = 'queued';

COMMENT ON COLUMN event_rsvps.user_id IS 'Immutable user DID; intentionally not a UUID foreign key.';
COMMENT ON COLUMN places.version IS 'Optimistic concurrency token for Studio mutations.';
COMMENT ON COLUMN appearances.version IS 'Optimistic concurrency token for Studio mutations.';

INSERT INTO schema_version(version, description)
SELECT 40, 'durable beta PostgreSQL repositories and invariants'
WHERE NOT EXISTS (SELECT 1 FROM schema_version WHERE version = 40);
