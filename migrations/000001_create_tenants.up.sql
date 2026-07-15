-- Enable UUID generation support.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ---------------------------------------------------------------------------
-- tenants
-- The root entity for the entire platform.  Every other table that holds
-- business data carries a tenant_id foreign key pointing here, providing
-- strict data isolation between businesses at the database level.
-- ---------------------------------------------------------------------------
CREATE TABLE tenants (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Human-readable identifiers.
    name           VARCHAR(255) NOT NULL,
    slug           VARCHAR(100) NOT NULL UNIQUE,   -- used in URLs / subdomains

    -- Contact & branding.
    email          VARCHAR(255) NOT NULL UNIQUE,
    phone          VARCHAR(50),
    logo_url       TEXT,
    website_url    TEXT,

    -- Subscription / lifecycle.
    plan           VARCHAR(50)  NOT NULL DEFAULT 'trial',
    is_active      BOOLEAN      NOT NULL DEFAULT TRUE,

    -- Audit timestamps.
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Index used by every tenant-scoped query that filters by slug or active state.
CREATE INDEX idx_tenants_slug     ON tenants (slug);
CREATE INDEX idx_tenants_is_active ON tenants (is_active);
