-- ---------------------------------------------------------------------------
-- Add visual branding fields to company_profiles
-- These fields power the white-label customer portal appearance.
-- ---------------------------------------------------------------------------
ALTER TABLE company_profiles
    ADD COLUMN IF NOT EXISTS favicon_url     TEXT,
    ADD COLUMN IF NOT EXISTS hero_image_url  TEXT,
    ADD COLUMN IF NOT EXISTS hero_eyebrow    VARCHAR(255),
    ADD COLUMN IF NOT EXISTS cover_image_url TEXT,
    ADD COLUMN IF NOT EXISTS tagline         VARCHAR(255),
    ADD COLUMN IF NOT EXISTS logo_url        TEXT;
