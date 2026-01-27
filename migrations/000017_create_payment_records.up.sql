-- Migration: Create payment_records table for tracking checkout sessions
-- Adds: payment_records table with status tracking and foreign keys

-- ============================================
-- Create payment_records table
-- ============================================

CREATE TABLE IF NOT EXISTS payment_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    amount BIGINT NOT NULL,
    fee BIGINT NOT NULL,
    user_did VARCHAR(255) NOT NULL,
    scene_id UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    event_id UUID REFERENCES events(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_payment_status CHECK (status IN ('pending', 'succeeded', 'failed', 'canceled')),
    CONSTRAINT chk_positive_amount CHECK (amount > 0),
    CONSTRAINT chk_non_negative_fee CHECK (fee >= 0)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_payment_records_session_id ON payment_records(session_id);
CREATE INDEX IF NOT EXISTS idx_payment_records_user_did ON payment_records(user_did);
CREATE INDEX IF NOT EXISTS idx_payment_records_scene_id ON payment_records(scene_id);
CREATE INDEX IF NOT EXISTS idx_payment_records_event_id ON payment_records(event_id) WHERE event_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_records_status ON payment_records(status);
CREATE INDEX IF NOT EXISTS idx_payment_records_created_at ON payment_records(created_at);

-- Add comments
COMMENT ON TABLE payment_records IS 'Tracks provisional payment records for Stripe Checkout Sessions';
COMMENT ON COLUMN payment_records.session_id IS 'Stripe Checkout Session ID';
COMMENT ON COLUMN payment_records.status IS 'Payment status: pending, succeeded, failed, canceled';
COMMENT ON COLUMN payment_records.amount IS 'Total amount in cents';
COMMENT ON COLUMN payment_records.fee IS 'Platform fee in cents';
COMMENT ON COLUMN payment_records.user_did IS 'Decentralized Identifier of user making payment';
