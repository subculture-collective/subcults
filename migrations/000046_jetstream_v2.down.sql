DELETE FROM schema_version WHERE version = 46;
DROP VIEW IF EXISTS jetstream_v2_projection_comparison;
ALTER TABLE backfill_checkpoints
    DROP COLUMN IF EXISTS rebuild_id,
    DROP COLUMN IF EXISTS target,
    DROP COLUMN IF EXISTS cursor_seq;
DROP TABLE IF EXISTS jetstream_v2_shadow_records;
DROP TABLE IF EXISTS jetstream_v2_shadow_failures;
DROP TABLE IF EXISTS jetstream_v2_shadow_reconciliations;
DROP TABLE IF EXISTS jetstream_v2_shadow_identities;
DROP TABLE IF EXISTS jetstream_v2_shadow_accounts;
DROP TABLE IF EXISTS jetstream_v2_reconciliations;
DROP TABLE IF EXISTS jetstream_v2_identities;
DROP TABLE IF EXISTS jetstream_v2_accounts;
DROP TABLE IF EXISTS jetstream_v2_cursors;
