ALTER TABLE payment_records
    ADD COLUMN IF NOT EXISTS appearance_id UUID REFERENCES appearances(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS tour_id UUID REFERENCES tours(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS signal_id UUID REFERENCES signals(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS delivery_id UUID REFERENCES deliveries(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS attribution_model TEXT,
    ADD COLUMN IF NOT EXISTS attribution_window_seconds BIGINT,
    ADD COLUMN IF NOT EXISTS provider_event_id TEXT,
    ADD COLUMN IF NOT EXISTS raw_payload_sha256 CHAR(64),
    ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'payment_records_attribution_window_nonnegative'
          AND conrelid = 'payment_records'::regclass
    ) THEN
        ALTER TABLE payment_records
            ADD CONSTRAINT payment_records_attribution_window_nonnegative
            CHECK (attribution_window_seconds IS NULL OR attribution_window_seconds >= 0);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'payment_records_payload_digest_format'
          AND conrelid = 'payment_records'::regclass
    ) THEN
        ALTER TABLE payment_records
            ADD CONSTRAINT payment_records_payload_digest_format
            CHECK (raw_payload_sha256 IS NULL OR raw_payload_sha256 ~ '^[0-9a-f]{64}$');
    END IF;
END $$;

ALTER TABLE webhook_events
    ADD COLUMN IF NOT EXISTS raw_payload_sha256 CHAR(64),
    ADD COLUMN IF NOT EXISTS received_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_payment_records_signal_id
    ON payment_records(signal_id) WHERE signal_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_records_delivery_id
    ON payment_records(delivery_id) WHERE delivery_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_records_appearance_id
    ON payment_records(appearance_id) WHERE appearance_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_records_tour_id
    ON payment_records(tour_id) WHERE tour_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_records_provider_event_id
    ON payment_records(provider_event_id) WHERE provider_event_id IS NOT NULL;

COMMENT ON COLUMN payment_records.attribution_model IS 'Named modeled relationship; never a claim of causal ground truth';
COMMENT ON COLUMN payment_records.attribution_window_seconds IS 'Explicit bounded attribution window used by the named model';
COMMENT ON COLUMN payment_records.raw_payload_sha256 IS 'SHA-256 audit digest; raw provider payload is not retained in this column';
