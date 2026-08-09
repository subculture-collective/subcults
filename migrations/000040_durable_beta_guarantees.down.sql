DROP INDEX IF EXISTS deliveries_dispatch_queue;
DROP INDEX IF EXISTS consent_events_effective_state;
DROP INDEX IF EXISTS entity_assertions_one_correction;

DROP TRIGGER IF EXISTS signal_revisions_immutable_delete ON signal_revisions;
DROP TRIGGER IF EXISTS signal_revisions_immutable_update ON signal_revisions;
DROP FUNCTION IF EXISTS reject_signal_revision_mutation();

-- Compound application identifiers cannot be safely narrowed back to UUID.

-- The DID-backed RSVP identity cannot be safely narrowed back to UUID without
-- potentially destroying valid data. The down migration intentionally leaves
-- it as TEXT. Version columns and compound Signal identifiers are also retained
-- so rollback/reapply cannot discard concurrency or provenance state.

DELETE FROM schema_version WHERE version = 40;
