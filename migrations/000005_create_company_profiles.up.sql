-- ---------------------------------------------------------------------------
-- company_profiles
-- Extended business profile for each tenant.  One-to-one with tenants.
-- Business admins manage this through the company settings surface.
-- ---------------------------------------------------------------------------
CREATE TABLE company_profiles (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID        NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,

    -- Business identity
    legal_name      VARCHAR(255),
    business_type   VARCHAR(50),        -- 'car_yard' | 'dealership' | 'rental' | 'service_center' | 'mixed'
    registration_no VARCHAR(100),
    tax_pin         VARCHAR(100),
    description     TEXT,

    -- Address
    country         VARCHAR(100),
    city            VARCHAR(100),
    address_line1   VARCHAR(255),
    address_line2   VARCHAR(255),
    postal_code     VARCHAR(20),

    -- Contact
    support_email   VARCHAR(255),
    support_phone   VARCHAR(50),
    whatsapp        VARCHAR(50),

    -- Social links (stored as key/value JSON)
    social_links    JSONB       NOT NULL DEFAULT '{}',

    -- Branding
    primary_color   VARCHAR(20),        -- hex, e.g. "#1A73E8"
    secondary_color VARCHAR(20),
    font_family     VARCHAR(100),

    -- Operating hours (stored as structured JSON)
    -- e.g. {"monday":{"open":"08:00","close":"17:00","closed":false}, ...}
    operating_hours JSONB       NOT NULL DEFAULT '{}',

    -- Module / feature flags for this tenant
    enable_hire     BOOLEAN     NOT NULL DEFAULT FALSE,
    enable_sales    BOOLEAN     NOT NULL DEFAULT FALSE,
    enable_service  BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Currency & locale
    currency        VARCHAR(10) NOT NULL DEFAULT 'KES',
    timezone        VARCHAR(50) NOT NULL DEFAULT 'Africa/Nairobi',

    -- Audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_company_profiles_tenant_id ON company_profiles (tenant_id);

-- Keep updated_at in sync automatically.
CREATE TRIGGER trg_company_profiles_updated_at
    BEFORE UPDATE ON company_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
