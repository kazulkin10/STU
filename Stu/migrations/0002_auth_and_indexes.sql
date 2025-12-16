-- Additional auth structures and performance indexes

-- Verification codes (hashed) for email confirmation
CREATE TABLE IF NOT EXISTS verification_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_verification_user_active ON verification_codes (user_id, expires_at) WHERE consumed_at IS NULL;

-- Session rotation/reuse detection
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS last_refresh_token_hash BYTEA,
    ADD COLUMN IF NOT EXISTS revoked_reason TEXT,
    ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh ON sessions(refresh_token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_last_refresh ON sessions(last_refresh_token_hash);

-- Messaging performance
CREATE INDEX IF NOT EXISTS idx_messages_dialog_created ON messages(dialog_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dialog_members_dialog ON dialog_members(dialog_id);
CREATE INDEX IF NOT EXISTS idx_dialog_members_user ON dialog_members(user_id);
