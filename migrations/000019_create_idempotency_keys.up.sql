-- Migration: Create idempotency_keys table for preventing duplicate payment operations
-- Adds: idempotency_keys table for tracking idempotent requests

-- ============================================
-- Create idempotency_keys table
-- ============================================

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key VARCHAR(64) PRIMARY KEY,
    method VARCHAR(10) NOT NULL,
    route VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payment_id UUID,
    response_hash VARCHAR(64) NOT NULL,
    status VARCHAR(50) NOT NULL,
    response_body TEXT NOT NULL,
    response_status_code INT NOT NULL,
    
    -- Constraints
    CONSTRAINT chk_idempotency_status CHECK (status IN ('completed', 'processing'))
);

-- Create index for cleanup queries (remove entries older than 24h)
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_created_at ON idempotency_keys(created_at);

-- Add comments
COMMENT ON TABLE idempotency_keys IS 'Stores idempotency keys to prevent duplicate payment operations';
COMMENT ON COLUMN idempotency_keys.key IS 'Client-provided idempotency key (max 64 chars)';
COMMENT ON COLUMN idempotency_keys.method IS 'HTTP method (POST, etc.)';
COMMENT ON COLUMN idempotency_keys.route IS 'API route path';
COMMENT ON COLUMN idempotency_keys.payment_id IS 'Associated payment record ID if applicable';
COMMENT ON COLUMN idempotency_keys.response_hash IS 'SHA256 hash of response body for validation';
COMMENT ON COLUMN idempotency_keys.status IS 'Status: completed (final), processing (in-progress)';
COMMENT ON COLUMN idempotency_keys.response_body IS 'Cached response body to return for duplicate requests';
COMMENT ON COLUMN idempotency_keys.response_status_code IS 'HTTP status code to return for duplicate requests';
