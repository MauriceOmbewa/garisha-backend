-- ---------------------------------------------------------------------------
-- vehicles
-- Core vehicle inventory for a tenant.  A vehicle can be used across hire,
-- sales, and service modules depending on the tenant's enabled features.
-- ---------------------------------------------------------------------------
CREATE TABLE vehicles (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Identity
    make        VARCHAR(100) NOT NULL,               -- e.g. "Toyota"
    model       VARCHAR(100) NOT NULL,               -- e.g. "Corolla"
    year        SMALLINT     NOT NULL,               -- e.g. 2021
    color       VARCHAR(50),
    vin         VARCHAR(50),                         -- Vehicle Identification Number
    plate_no    VARCHAR(30),                         -- registration / number plate

    -- Classification
    vehicle_type   VARCHAR(50) NOT NULL DEFAULT 'sedan',
    -- 'sedan' | 'suv' | 'truck' | 'van' | 'bus' | 'pickup' | 'motorcycle' | 'other'

    -- Lifecycle status
    status      VARCHAR(30) NOT NULL DEFAULT 'available',
    -- 'available' | 'hired' | 'sold' | 'under_service' | 'inactive'

    -- Odometer / fuel
    mileage     INTEGER,                             -- kilometres
    fuel_type   VARCHAR(30),                         -- 'petrol' | 'diesel' | 'electric' | 'hybrid'

    -- Pricing hints (nullable — exact pricing lives in hire/sale records)
    daily_rate  NUMERIC(12, 2),                      -- hire rate per day
    sale_price  NUMERIC(12, 2),                      -- asking price for sale

    -- Images — ordered list of URLs stored as a JSONB array
    images      JSONB        NOT NULL DEFAULT '[]',

    -- Free-form notes
    notes       TEXT,

    -- Audit
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vehicles_tenant_id ON vehicles (tenant_id);
CREATE INDEX idx_vehicles_status    ON vehicles (tenant_id, status);
CREATE INDEX idx_vehicles_type      ON vehicles (tenant_id, vehicle_type);

-- Keep updated_at in sync.
CREATE TRIGGER trg_vehicles_updated_at
    BEFORE UPDATE ON vehicles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
