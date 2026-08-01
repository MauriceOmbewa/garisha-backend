-- ---------------------------------------------------------------------------
-- branches
-- A tenant may operate one or more physical locations (branches / yards).
-- All core data tables carry an optional branch_id so records can be scoped
-- to a specific location.  NULL branch_id means the record belongs to the
-- tenant at large (useful during migration or for records that genuinely
-- span branches, e.g. inter-branch vehicle transfers).
-- ---------------------------------------------------------------------------
CREATE TABLE branches (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(100) NOT NULL,           -- unique within tenant
    city        VARCHAR(100),
    address     TEXT,
    phone       VARCHAR(50),
    email       VARCHAR(255),
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    is_default  BOOLEAN      NOT NULL DEFAULT FALSE, -- the "main" branch

    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_branches_tenant_slug UNIQUE (tenant_id, slug)
);

CREATE INDEX idx_branches_tenant_id ON branches (tenant_id);

CREATE TRIGGER trg_branches_updated_at
    BEFORE UPDATE ON branches
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Add branch_id to all core data tables.
-- NULL = tenant-wide / not yet assigned to a branch.
-- ---------------------------------------------------------------------------
ALTER TABLE vehicles       ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE hire_bookings  ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE vehicle_sales  ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE service_jobs   ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE customers      ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE finance_records ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE payments       ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
ALTER TABLE inventory_items ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;

-- Indexes for branch-scoped queries
CREATE INDEX IF NOT EXISTS idx_vehicles_branch_id        ON vehicles        (branch_id);
CREATE INDEX IF NOT EXISTS idx_hire_bookings_branch_id   ON hire_bookings   (branch_id);
CREATE INDEX IF NOT EXISTS idx_vehicle_sales_branch_id   ON vehicle_sales   (branch_id);
CREATE INDEX IF NOT EXISTS idx_service_jobs_branch_id    ON service_jobs    (branch_id);
CREATE INDEX IF NOT EXISTS idx_customers_branch_id       ON customers       (branch_id);
CREATE INDEX IF NOT EXISTS idx_finance_records_branch_id ON finance_records (branch_id);
CREATE INDEX IF NOT EXISTS idx_payments_branch_id        ON payments        (branch_id);
CREATE INDEX IF NOT EXISTS idx_inventory_items_branch_id ON inventory_items (branch_id);
