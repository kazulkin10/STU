-- Ensure reports table has reported_user_id for existing databases
ALTER TABLE IF EXISTS reports
    ADD COLUMN IF NOT EXISTS reported_user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Recreate index safely on reported_user_id + created_at
DROP INDEX IF EXISTS idx_reports_reported_created;
CREATE INDEX IF NOT EXISTS idx_reports_reported_created ON reports(reported_user_id, created_at DESC);
