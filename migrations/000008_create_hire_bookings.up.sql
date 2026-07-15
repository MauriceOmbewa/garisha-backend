-- ---------------------------------------------------------------------------
-- hire_bookings
-- Car-hire booking records, tenant-scoped.  Each booking links a vehicle to
-- a customer for a given hire period, records the agreed pricing, and tracks
-- the full status lifecycle.
-- ---------------------------------------------------------------------------
CREATE TABLE hire_bookings (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id)   ON DELETE CASCADE,
    vehicle_id  UUID         NOT NULL REFERENCES vehicles(id)  ON DELETE RESTRICT,
    customer_id UUID         NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,

    -- Hire period
    start_date  DATE         NOT NULL,
    end_date    DATE         NOT NULL,
    pickup_time TIME,                        -- optional precise pickup time
    return_time TIME,                        -- optional agreed return time

    -- Actual dates (set when vehicle is physically collected / returned)
    actual_start TIMESTAMPTZ,
    actual_end   TIMESTAMPTZ,

    -- Pricing
    daily_rate      NUMERIC(12, 2) NOT NULL,
    total_days      INTEGER        NOT NULL,
    total_amount    NUMERIC(12, 2) NOT NULL,
    deposit_amount  NUMERIC(12, 2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    final_amount    NUMERIC(12, 2) NOT NULL, -- total_amount - discount_amount

    -- Status lifecycle
    -- 'pending' → 'confirmed' → 'active' → 'completed' | 'cancelled'
    status      VARCHAR(30)  NOT NULL DEFAULT 'pending',

    -- Pickup / return locations
    pickup_location VARCHAR(255),
    return_location VARCHAR(255),

    -- Odometer readings
    mileage_out INTEGER,   -- km at vehicle pickup
    mileage_in  INTEGER,   -- km at vehicle return

    -- Staff who handled the booking
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Notes
    notes       TEXT,

    -- Audit
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_hire_dates        CHECK (end_date >= start_date),
    CONSTRAINT chk_hire_daily_rate   CHECK (daily_rate >= 0),
    CONSTRAINT chk_hire_total_days   CHECK (total_days > 0),
    CONSTRAINT chk_hire_total_amount CHECK (total_amount >= 0),
    CONSTRAINT chk_hire_final_amount CHECK (final_amount >= 0),
    CONSTRAINT chk_hire_status       CHECK (status IN ('pending','confirmed','active','completed','cancelled'))
);

CREATE INDEX idx_hire_bookings_tenant_id   ON hire_bookings (tenant_id);
CREATE INDEX idx_hire_bookings_vehicle_id  ON hire_bookings (vehicle_id);
CREATE INDEX idx_hire_bookings_customer_id ON hire_bookings (customer_id);
CREATE INDEX idx_hire_bookings_status      ON hire_bookings (tenant_id, status);
CREATE INDEX idx_hire_bookings_dates       ON hire_bookings (tenant_id, start_date, end_date);

CREATE TRIGGER trg_hire_bookings_updated_at
    BEFORE UPDATE ON hire_bookings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
