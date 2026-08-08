CREATE TABLE web_push_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint_ciphertext BYTEA NOT NULL,
    endpoint_hmac CHAR(64) NOT NULL,
    p256dh_ciphertext BYTEA NOT NULL,
    auth_ciphertext BYTEA NOT NULL,
    user_agent_hash CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX web_push_subscriptions_active_endpoint
    ON web_push_subscriptions(endpoint_hmac) WHERE revoked_at IS NULL;
CREATE INDEX web_push_subscriptions_active_user
    ON web_push_subscriptions(user_id) WHERE revoked_at IS NULL;

COMMENT ON TABLE web_push_subscriptions IS 'Encrypted browser delivery coordinates; browser permission is not Signal consent.';
