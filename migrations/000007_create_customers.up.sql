-- ---------------------------------------------------------------------------
-- customers
-- Customer profiles, tenant-scoped.  A customer can be linked to bookings,
-- sales, and service jobs.  They may or may not have a platform user account
-- (walk-in customers have no login).
-- ---------------------------------------------------------------------------
CREATE TABLE customers (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Optional link to a platform user account (Google-authenticated).
    -- NULL for walk-in / manually created customers.
    user_id     UUID         REFERENCES users(id) ON DELETE SET NULL,

    -- Identity
    full_name   VARCHAR(255) NOT NULL,
    email       VARCHAR(255),
    phone       VARCHAR(50),
    id_number   VARCHAR(100),       -- national ID / passport
    id_type     VARCHAR(30),        -- 'national_id' | 'passport' | 'driving_license' | 'other'

    -- Address
    country     VARCHAR(100),
    city        VARCHAR(100),
    address     TEXT,

    -- Lifecycle
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,

    -- Free-form notes
    notes       TEXT,

    -- Audit
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- Email unique per tenant (nullable so walk-ins without email can coexist).
    CONSTRAINT uq_customers_tenant_email UNIQUE (tenant_id, email)
);

CREATE INDEX idx_customers_tenant_id ON customers (tenant_id);
CREATE INDEX idx_customers_user_id   ON customers (user_id);
CREATE INDEX idx_customers_email     ON customers (tenant_id, email);
CREATE INDEX idx_customers_phone     ON customers (tenant_id, phone);

CREATE TRIGGER trg_customers_updated_at
    BEFORE UPDATE ON customers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
