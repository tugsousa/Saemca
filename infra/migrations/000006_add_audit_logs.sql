-- GDPR compliance: every read or write of health-sensitive data must be logged here.
-- actor_id is nullable to support system-initiated actions (e.g. cron jobs).
-- This table is append-only — rows are never updated or deleted.

CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID REFERENCES users(id), -- NULL for system actions
    action      TEXT NOT NULL,             -- e.g. VIEW_HEALTH_DATA, UPDATE_PROFILE, ABORT_VISIT
    target_type TEXT NOT NULL,             -- e.g. PROFILE, BOOKING, DOCUMENT
    target_id   UUID NOT NULL,
    ip_address  TEXT,
    metadata    JSONB,                     -- Any extra context (before/after values, etc.)
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_actor_id   ON audit_logs(actor_id);
CREATE INDEX idx_audit_logs_target_id  ON audit_logs(target_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
