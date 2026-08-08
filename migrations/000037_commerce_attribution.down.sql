DROP INDEX IF EXISTS idx_payment_records_provider_event_id;
DROP INDEX IF EXISTS idx_payment_records_tour_id;
DROP INDEX IF EXISTS idx_payment_records_appearance_id;
DROP INDEX IF EXISTS idx_payment_records_delivery_id;
DROP INDEX IF EXISTS idx_payment_records_signal_id;

ALTER TABLE webhook_events
    DROP COLUMN IF EXISTS received_at,
    DROP COLUMN IF EXISTS raw_payload_sha256;

ALTER TABLE payment_records
    DROP CONSTRAINT IF EXISTS payment_records_payload_digest_format,
    DROP CONSTRAINT IF EXISTS payment_records_attribution_window_nonnegative,
    DROP COLUMN IF EXISTS received_at,
    DROP COLUMN IF EXISTS raw_payload_sha256,
    DROP COLUMN IF EXISTS provider_event_id,
    DROP COLUMN IF EXISTS attribution_window_seconds,
    DROP COLUMN IF EXISTS attribution_model,
    DROP COLUMN IF EXISTS delivery_id,
    DROP COLUMN IF EXISTS signal_id,
    DROP COLUMN IF EXISTS tour_id,
    DROP COLUMN IF EXISTS appearance_id;
