-- Dispute resolution tables.
-- Full business logic spec: docs/disputes.md
--
-- Key rules:
-- - disputes_lost on users increments only when a dispute resolves AGAINST that user
-- - Payment stays AUTHORIZED for the entire dispute lifecycle
-- - Mutual resolution requires explicit acceptance from both parties (mutual_accepted_by_client + pro)
-- - Appeals go to a DIFFERENT admin than the original resolver

-- ── disputes ────────────────────────────────────────────────────────────────

CREATE TABLE disputes (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id           UUID NOT NULL REFERENCES bookings(id),
    opened_by            UUID NOT NULL REFERENCES users(id),
    opened_by_role       TEXT NOT NULL CHECK (opened_by_role IN ('CLIENT', 'PRO')),
    reason_category      TEXT NOT NULL CHECK (reason_category IN (
                             -- Client reasons
                             'NO_SHOW', 'INCOMPLETE_SERVICE', 'SAFETY_CONCERN',
                             'UNAUTHORIZED_ACTION',
                             -- Pro reasons
                             'CLIENT_ABUSIVE', 'FALSE_DISPUTE', 'PAYMENT_ISSUE',
                             -- Shared
                             'OTHER'
                         )),
    description          TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'OPEN'
                             CHECK (status IN ('OPEN', 'UNDER_REVIEW', 'RESOLVED', 'APPEALED')),

    -- Resolution fields (null until resolved)
    resolution           TEXT CHECK (resolution IN ('PRO_WIN', 'CLIENT_WIN', 'SPLIT', 'MUTUAL')),
    split_pro_percent    INTEGER CHECK (split_pro_percent BETWEEN 0 AND 100),
    decision_message     TEXT,   -- Shown to both parties on resolution
    admin_notes          TEXT,   -- Internal only — never shown to parties
    resolved_by          UUID REFERENCES users(id),
    resolved_at          TIMESTAMP WITH TIME ZONE,

    -- Mutual resolution — requires explicit in-app confirmation from both parties
    mutual_accepted_by_client  BOOLEAN DEFAULT FALSE,
    mutual_accepted_by_pro     BOOLEAN DEFAULT FALSE,

    -- SLA tracking
    evidence_deadline    TIMESTAMP WITH TIME ZONE NOT NULL,  -- opened_at + 48h
    resolution_deadline  TIMESTAMP WITH TIME ZONE NOT NULL,  -- opened_at + 5 business days
    sla_paused           BOOLEAN DEFAULT FALSE,
    sla_paused_at        TIMESTAMP WITH TIME ZONE,

    -- Appeal fields (null until appeal is filed)
    appeal_status        TEXT CHECK (appeal_status IN ('REQUESTED', 'UPHELD', 'OVERTURNED')),
    appeal_opened_at     TIMESTAMP WITH TIME ZONE,
    appeal_resolved_by   UUID REFERENCES users(id),  -- Must differ from resolved_by
    appeal_resolved_at   TIMESTAMP WITH TIME ZONE,

    created_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_disputes_booking_id ON disputes(booking_id);
CREATE INDEX idx_disputes_status     ON disputes(status);
CREATE INDEX idx_disputes_opened_by  ON disputes(opened_by);

-- ── dispute_evidence ────────────────────────────────────────────────────────

CREATE TABLE dispute_evidence (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispute_id    UUID NOT NULL REFERENCES disputes(id) ON DELETE CASCADE,
    submitted_by  UUID NOT NULL REFERENCES users(id),
    statement     TEXT,           -- Written statement from the party
    storage_key   TEXT,           -- Optional file in R2/MinIO
    mime_type     TEXT,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_dispute_evidence_dispute_id ON dispute_evidence(dispute_id);

-- ── dispute_messages ────────────────────────────────────────────────────────
-- Private per-party threads: Admin ↔ Client and Admin ↔ Pro.
-- Parties cannot see each other's thread (enforce at application layer).

CREATE TABLE dispute_messages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispute_id    UUID NOT NULL REFERENCES disputes(id) ON DELETE CASCADE,
    sender_id     UUID NOT NULL REFERENCES users(id),
    recipient_id  UUID NOT NULL REFERENCES users(id),
    message       TEXT NOT NULL,
    is_internal   BOOLEAN DEFAULT FALSE,  -- TRUE = admin-only note, never shown to parties
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_dispute_messages_dispute_id ON dispute_messages(dispute_id);

-- ── users table additions ───────────────────────────────────────────────────

ALTER TABLE users
    ADD COLUMN disputes_lost        INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN manual_review_flag   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN manual_review_reason TEXT;
