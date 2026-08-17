ALTER TABLE company_profiles
    DROP COLUMN IF EXISTS favicon_url,
    DROP COLUMN IF EXISTS hero_image_url,
    DROP COLUMN IF EXISTS hero_eyebrow,
    DROP COLUMN IF EXISTS cover_image_url,
    DROP COLUMN IF EXISTS tagline,
    DROP COLUMN IF EXISTS logo_url;
