-- ---------------------------------------------------------------------------
-- vehicle_sales
-- Records each vehicle sale transaction, linking a vehicle to a buyer
-- (customer), capturing agreed pricing, payment terms, and progressing
-- through a status lifecycle:
--   pending → reserved → completed | cancelled
-- ---------------------------------------------------------------------------
CREATE TABLE vehicle_sales (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id)   ON DELETE CASCADE,
    vehicle_id  UUID         NOT NULL REFERENCES vehicles(id)  ON DELETE RESTRICT,
    customer_id UUID         NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,

    -- Pricing
    asking_price    NUMERIC(12, 2) NOT NULL,           -- listed price at time of sale
    agreed_price    NUMERIC(12, 2) NOT NULL,           -- negotiated sale price
    deposit_amount  NUMERIC(12, 2) NOT NULL DEFAULT 0, -- deposit / down-payment received
    discount_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    final_amount    NUMERIC(12, 2) NOT NULL,           -- agreed_price - discount_amount

    -- Payment terms
    payment_method  VARCHAR(50),   -- 'cash' | 'mpesa' | 'bank_transfer' | 'finance' | 'other'
    payment_terms   VARCHAR(100),  -- e.g. "50% deposit, balance on delivery"

    -- Sale date (when the deal was agreed — may differ from vehicle handover)
    sale_date       DATE           NOT NULL DEFAULT CURRENT_DATE,

    -- Physical handover timestamp (set when vehicle keys are handed over)
    handover_at     TIMESTAMPTZ,

    -- Status lifecycle
    -- 'pending' → 'reserved' → 'completed' | 'cancelled'
    status          VARCHAR(30)    NOT NULL DEFAULT 'pending',

    -- Documents & reference
    invoice_number  VARCHAR(100),   -- dealership invoice / receipt number
    contract_ref    VARCHAR(100),   -- sale contract reference

    -- Staff who recorded the sale
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Notes
    notes           TEXT,

    -- Audit
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_sale_asking_price   CHECK (asking_price   >= 0),
    CONSTRAINT chk_sale_agreed_price   CHECK (agreed_price   >= 0),
    CONSTRAINT chk_sale_deposit        CHECK (deposit_amount >= 0),
    CONSTRAINT chk_sale_discount       CHECK (discount_amount >= 0),
    CONSTRAINT chk_sale_final_amount   CHECK (final_amount   >= 0),
    CONSTRAINT chk_sale_status         CHECK (status IN ('pending','reserved','completed','cancelled'))
);

CREATE INDEX idx_vehicle_sales_tenant_id   ON vehicle_sales (tenant_id);
CREATE INDEX idx_vehicle_sales_vehicle_id  ON vehicle_sales (vehicle_id);
CREATE INDEX idx_vehicle_sales_customer_id ON vehicle_sales (customer_id);
CREATE INDEX idx_vehicle_sales_status      ON vehicle_sales (tenant_id, status);
CREATE INDEX idx_vehicle_sales_sale_date   ON vehicle_sales (tenant_id, sale_date);

CREATE TRIGGER trg_vehicle_sales_updated_at
    BEFORE UPDATE ON vehicle_sales
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
