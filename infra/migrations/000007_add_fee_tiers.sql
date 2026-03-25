-- Platform fee tiers. Each professional is assigned one tier.
-- The "Founding 50" tier grants 0% fees as an early-adopter reward.

CREATE TABLE fee_tiers (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                 TEXT NOT NULL UNIQUE,       -- e.g. 'FOUNDING_50', 'STANDARD'
    platform_fee_percent INTEGER NOT NULL            -- Integer percentage, e.g. 15 = 15%
                             CHECK (platform_fee_percent BETWEEN 0 AND 100),
    description          TEXT,
    created_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add fee tier and verification fields to professionals
ALTER TABLE professionals
    ADD COLUMN fee_tier_id  UUID REFERENCES fee_tiers(id),
    ADD COLUMN verified      BOOLEAN DEFAULT FALSE,
    ADD COLUMN verified_at   TIMESTAMP WITH TIME ZONE;

-- Seed the default tiers
INSERT INTO fee_tiers (name, platform_fee_percent, description) VALUES
    ('FOUNDING_50', 0,  'Early adopter professionals — 0% platform fee for life'),
    ('STANDARD',    15, 'Standard platform fee');
