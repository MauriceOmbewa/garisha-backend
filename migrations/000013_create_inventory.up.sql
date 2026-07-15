-- ---------------------------------------------------------------------------
-- inventory_items
-- Parts, consumables, tools and other stock items held by a tenant.
-- Each item tracks current stock quantity and a reorder threshold.
-- ---------------------------------------------------------------------------
CREATE TABLE inventory_items (
    id              UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID          NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Identity
    name            VARCHAR(255)  NOT NULL,
    sku             VARCHAR(100),              -- stock-keeping unit / part number
    description     TEXT,

    -- Classification
    category        VARCHAR(100),             -- e.g. 'Engine', 'Brakes', 'Lubricants'
    unit            VARCHAR(30)   NOT NULL DEFAULT 'piece',
    -- 'piece' | 'litre' | 'kg' | 'metre' | 'set' | 'box' | 'other'

    -- Stock levels
    quantity        NUMERIC(12, 3) NOT NULL DEFAULT 0,
    reorder_level   NUMERIC(12, 3) NOT NULL DEFAULT 0,  -- alert threshold
    reorder_qty     NUMERIC(12, 3) NOT NULL DEFAULT 0,  -- suggested reorder quantity

    -- Pricing
    unit_cost       NUMERIC(12, 2) NOT NULL DEFAULT 0,  -- cost price per unit
    selling_price   NUMERIC(12, 2) NOT NULL DEFAULT 0,  -- sale/charge-out price

    -- Lifecycle
    is_active       BOOLEAN        NOT NULL DEFAULT TRUE,

    -- Supplier info (optional)
    supplier_name   VARCHAR(255),
    supplier_phone  VARCHAR(50),
    supplier_email  VARCHAR(255),

    -- Notes
    notes           TEXT,

    -- Audit
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_inventory_quantity      CHECK (quantity      >= 0),
    CONSTRAINT chk_inventory_reorder_level CHECK (reorder_level >= 0),
    CONSTRAINT chk_inventory_unit_cost     CHECK (unit_cost     >= 0),
    CONSTRAINT chk_inventory_selling_price CHECK (selling_price >= 0),
    CONSTRAINT uq_inventory_sku            UNIQUE (tenant_id, sku)
);

CREATE INDEX idx_inventory_items_tenant_id ON inventory_items (tenant_id);
CREATE INDEX idx_inventory_items_sku       ON inventory_items (tenant_id, sku);
CREATE INDEX idx_inventory_items_category  ON inventory_items (tenant_id, category);
-- Partial index for quick reorder alert queries.
CREATE INDEX idx_inventory_items_reorder   ON inventory_items (tenant_id)
    WHERE quantity <= reorder_level AND is_active = TRUE;

CREATE TRIGGER trg_inventory_items_updated_at
    BEFORE UPDATE ON inventory_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- inventory_usage
-- Records every stock movement: consumption against a service job,
-- manual adjustments (stock-take corrections, write-offs), and receipts
-- (stock coming in from a supplier).
-- ---------------------------------------------------------------------------
CREATE TABLE inventory_usage (
    id          UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID          NOT NULL REFERENCES tenants(id)        ON DELETE CASCADE,
    item_id     UUID          NOT NULL REFERENCES inventory_items(id) ON DELETE RESTRICT,

    -- Movement type
    movement    VARCHAR(20)   NOT NULL,
    -- 'usage'      – consumed in a service job
    -- 'adjustment' – manual stock correction (positive or negative)
    -- 'receipt'    – stock received from supplier

    -- Signed quantity: negative for consumption/write-off, positive for receipt/adjustment
    quantity    NUMERIC(12, 3) NOT NULL,

    -- Optional link to a service job item that triggered this usage
    service_job_id      UUID  REFERENCES service_jobs(id)      ON DELETE SET NULL,
    service_job_item_id UUID  REFERENCES service_job_items(id) ON DELETE SET NULL,

    -- Unit cost at time of movement (for FIFO / average cost tracking)
    unit_cost   NUMERIC(12, 2) NOT NULL DEFAULT 0,

    -- Reference (PO number, adjustment reason, delivery note, etc.)
    reference   VARCHAR(200),
    notes       TEXT,

    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_inventory_usage_movement CHECK (
        movement IN ('usage','adjustment','receipt')
    ),
    CONSTRAINT chk_inventory_usage_qty_nonzero CHECK (quantity <> 0)
);

CREATE INDEX idx_inventory_usage_tenant_id ON inventory_usage (tenant_id);
CREATE INDEX idx_inventory_usage_item_id   ON inventory_usage (item_id);
CREATE INDEX idx_inventory_usage_job_id    ON inventory_usage (service_job_id) WHERE service_job_id IS NOT NULL;
CREATE INDEX idx_inventory_usage_created   ON inventory_usage (tenant_id, created_at);

-- Trigger: keep inventory_items.quantity in sync after every usage record insert.
CREATE OR REPLACE FUNCTION apply_inventory_movement()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE inventory_items
    SET    quantity   = quantity + NEW.quantity,
           updated_at = NOW()
    WHERE  id = NEW.item_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_inventory_apply_movement
    AFTER INSERT ON inventory_usage
    FOR EACH ROW EXECUTE FUNCTION apply_inventory_movement();
