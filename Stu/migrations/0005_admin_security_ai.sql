-- Restructure reports to final shape with AI verdicts
DROP TABLE IF EXISTS reports CASCADE;

CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dialog_id UUID REFERENCES dialogs(id) ON DELETE SET NULL,
    message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open', -- open/closed
    ai_verdict TEXT,
    ai_confidence DOUBLE PRECISION,
    ai_notes TEXT,
    analyzed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reports_status_created_at ON reports(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reports_reported_created_at ON reports(reported_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reports_reporter_created_at ON reports(reporter_id, created_at DESC);

-- Admin security
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS admin_totp_secret TEXT;

CREATE TABLE IF NOT EXISTS admin_auth_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL, -- password_ok, totp_ok, completed
    totp_verified BOOLEAN NOT NULL DEFAULT FALSE,
    email_code TEXT,
    email_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_auth_sessions_token ON admin_auth_sessions(session_token);
