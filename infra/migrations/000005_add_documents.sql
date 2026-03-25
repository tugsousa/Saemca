-- Stores professional onboarding document metadata.
-- The actual file lives in Cloudflare R2 (or MinIO in dev).
-- storage_key is the object key used to retrieve/delete the file from R2.

CREATE TABLE documents (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_type     TEXT NOT NULL CHECK (document_type IN ('CEDULA', 'INSURANCE', 'OTHER')),
    storage_key       TEXT NOT NULL,       -- Object key in R2/MinIO
    original_filename TEXT NOT NULL,       -- Shown in the admin verification UI
    mime_type         TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'PENDING'
                          CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    reviewed_by       UUID REFERENCES users(id), -- Admin user who reviewed
    reviewed_at       TIMESTAMP WITH TIME ZONE,
    rejection_reason  TEXT,
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_documents_user_id ON documents(user_id);
CREATE INDEX idx_documents_status  ON documents(status);
