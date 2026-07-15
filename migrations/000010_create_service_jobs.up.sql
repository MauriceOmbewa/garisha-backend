-- ---------------------------------------------------------------------------
-- service_jobs
-- A service job records a vehicle coming in for inspection, repair, or
-- maintenance.  It is optionally linked to a customer (owner / drop-off
-- contact) and progresses through a status lifecycle:
--   pending → in_progress → awaiting_parts → completed | cancelled
--
-- service_job_items records individual tasks / parts within a job,
-- each with its own labour or parts cost.
-- ---------------------------------------------------------------------------

CREATE TABLE service_jobs (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id    UUID         NOT NULL REFERENCES tenants(id)   ON DELETE CASCADE,
    vehicle_id   UUID         NOT NULL REFERENCES vehicles(id)  ON DELETE RESTRICT,

    -- Optional: customer who dropped the vehicle off (may be null for fleet jobs)
    customer_id  UUID         REFERENCES customers(id) ON DELETE SET NULL,

    -- Assigned mechanic (platform user with mechanic / technician role)
    mechanic_id  UUID         REFERENCES users(id) ON DELETE SET NULL,

    -- Job classification
    job_type     VARCHAR(50)  NOT NULL DEFAULT 'general',
    -- 'general' | 'repair' | 'maintenance' | 'inspection' | 'bodywork' | 'electrical' | 'other'

    -- Key dates
    received_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),   -- when vehicle was booked/received
    due_date     DATE,                                   -- promised completion date
    completed_at TIMESTAMPTZ,                            -- actual completion timestamp

    -- Odometer at intake
    mileage_in   INTEGER,

    -- Lifecycle status
    -- 'pending' → 'in_progress' → 'awaiting_parts' → 'completed' | 'cancelled'
    status       VARCHAR(30)  NOT NULL DEFAULT 'pending',

    -- Pricing totals (summed from job items, stored for fast reads)
    labour_total NUMERIC(12, 2) NOT NULL DEFAULT 0,
    parts_total  NUMERIC(12, 2) NOT NULL DEFAULT 0,
    total_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    final_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,

    -- Staff who created the job record
    created_by   UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Notes visible to customer
    customer_notes TEXT,
    -- Internal workshop notes
    internal_notes TEXT,

    -- Audit
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_service_status CHECK (
        status IN ('pending','in_progress','awaiting_parts','completed','cancelled')
    ),
    CONSTRAINT chk_service_labour_total  CHECK (labour_total  >= 0),
    CONSTRAINT chk_service_parts_total   CHECK (parts_total   >= 0),
    CONSTRAINT chk_service_total_amount  CHECK (total_amount  >= 0),
    CONSTRAINT chk_service_discount      CHECK (discount_amount >= 0),
    CONSTRAINT chk_service_final_amount  CHECK (final_amount  >= 0)
);

CREATE INDEX idx_service_jobs_tenant_id   ON service_jobs (tenant_id);
CREATE INDEX idx_service_jobs_vehicle_id  ON service_jobs (vehicle_id);
CREATE INDEX idx_service_jobs_customer_id ON service_jobs (customer_id);
CREATE INDEX idx_service_jobs_mechanic_id ON service_jobs (mechanic_id);
CREATE INDEX idx_service_jobs_status      ON service_jobs (tenant_id, status);
CREATE INDEX idx_service_jobs_received_at ON service_jobs (tenant_id, received_at);

CREATE TRIGGER trg_service_jobs_updated_at
    BEFORE UPDATE ON service_jobs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- service_job_items
-- Individual line-items within a service job (labour tasks or parts used).
-- ---------------------------------------------------------------------------

CREATE TABLE service_job_items (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id      UUID         NOT NULL REFERENCES service_jobs(id) ON DELETE CASCADE,
    tenant_id   UUID         NOT NULL REFERENCES tenants(id)      ON DELETE CASCADE,

    -- Item classification
    item_type   VARCHAR(20)  NOT NULL DEFAULT 'labour',
    -- 'labour' | 'part' | 'consumable' | 'other'

    description VARCHAR(500) NOT NULL,

    -- Quantity & pricing
    quantity    NUMERIC(10, 3) NOT NULL DEFAULT 1,
    unit_price  NUMERIC(12, 2) NOT NULL DEFAULT 0,
    total_price NUMERIC(12, 2) NOT NULL DEFAULT 0, -- quantity * unit_price

    -- Optional part reference (links to future inventory module)
    part_number VARCHAR(100),

    -- Audit
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_item_type       CHECK (item_type IN ('labour','part','consumable','other')),
    CONSTRAINT chk_item_quantity   CHECK (quantity   >  0),
    CONSTRAINT chk_item_unit_price CHECK (unit_price >= 0),
    CONSTRAINT chk_item_total      CHECK (total_price >= 0)
);

CREATE INDEX idx_svc_job_items_job_id    ON service_job_items (job_id);
CREATE INDEX idx_svc_job_items_tenant_id ON service_job_items (tenant_id);

CREATE TRIGGER trg_service_job_items_updated_at
    BEFORE UPDATE ON service_job_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
