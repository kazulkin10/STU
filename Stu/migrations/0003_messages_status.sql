-- Delivery status and access token index

CREATE TABLE IF NOT EXISTS message_deliveries (
    message_id BIGINT REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_message_reads_message ON message_reads (message_id);
CREATE INDEX IF NOT EXISTS idx_message_deliveries_message ON message_deliveries (message_id);

-- Faster access token lookup
CREATE INDEX IF NOT EXISTS idx_sessions_access ON sessions(access_token_hash);
