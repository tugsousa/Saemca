-- Materials are the checklist items a client must confirm they possess
-- before a booking can be created. This feeds the "Material Gatekeeper" flow.
-- Each row belongs to a catalog service.

CREATE TABLE materials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    catalog_id  UUID NOT NULL REFERENCES catalog(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,        -- e.g. "Sterile gloves (size M)"
    description TEXT,                 -- Optional clarification shown to the client
    is_required BOOLEAN DEFAULT TRUE, -- If false, it's a recommendation, not mandatory
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_materials_catalog_id ON materials(catalog_id);
