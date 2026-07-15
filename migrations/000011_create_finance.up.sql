-- ---------------------------------------------------------------------------
-- finance_categories
-- Tenant-defined labels used to group income and expense records.
-- Seeded with common defaults; tenants can add their own.
-- ---------------------------------------------------------------------------
CREATE TABLE finance_categories (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(10)  NOT NULL,          -- 'income' | 'expense'
    description TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_finance_category_type CHECK (type IN ('income','expense')),
    CONSTRAINT uq_finance_category_name  UNIQUE (tenant_id, name, type)
);

CREATE INDEX idx_finance_categories_tenant_id ON finance_categories (tenant_id);
CREATE INDEX idx_finance_categories_type      ON finance_categories (tenant_id, type);

CREATE TRIGGER trg_finance_categories_updated_at
    BEFORE UPDATE ON finance_categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- finance_records
-- Individual income or expense entries in the tenant ledger.
-- Each record can optionally reference a hire booking, sale, or service job
-- as its source transaction (for traceability).
-- ---------------------------------------------------------------------------
CREATE TABLE finance_records (
    id              UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID          NOT NULL REFERENCES tenants(id)           ON DELETE CASCADE,
    category_id     UUID          NOT NULL REFERENCES finance_categories(id) ON DELETE RESTRICT,

    -- Transaction type
    type            VARCHAR(10)   NOT NULL,   -- 'income' | 'expense'

    -- Amount (always positive; type field determines direction)
    amount          NUMERIC(14, 2) NOT NULL,

    -- Reference to source transaction (all optional)
    hire_booking_id UUID          REFERENCES hire_bookings(id)  ON DELETE SET NULL,
    sale_id         UUID          REFERENCES vehicle_sales(id)  ON DELETE SET NULL,
    service_job_id  UUID          REFERENCES service_jobs(id)   ON DELETE SET NULL,

    -- Human-readable description / memo
    description     VARCHAR(500)  NOT NULL,

    -- Date the transaction occurred (may differ from created_at)
    transaction_date DATE         NOT NULL DEFAULT CURRENT_DATE,

    -- Payment channel
    payment_method  VARCHAR(50),  -- 'cash' | 'mpesa' | 'bank_transfer' | 'card' | 'other'

    -- Reference number (receipt, bank ref, M-PESA code, etc.)
    reference       VARCHAR(200),

    -- Staff who entered the record
    created_by      UUID          REFERENCES users(id) ON DELETE SET NULL,

    -- Notes
    notes           TEXT,

    -- Audit
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_finance_record_type   CHECK (type IN ('income','expense')),
    CONSTRAINT chk_finance_record_amount CHECK (amount > 0)
);

CREATE INDEX idx_finance_records_tenant_id        ON finance_records (tenant_id);
CREATE INDEX idx_finance_records_category_id      ON finance_records (category_id);
CREATE INDEX idx_finance_records_type             ON finance_records (tenant_id, type);
CREATE INDEX idx_finance_records_transaction_date ON finance_records (tenant_id, transaction_date);
CREATE INDEX idx_finance_records_hire_booking_id  ON finance_records (hire_booking_id) WHERE hire_booking_id IS NOT NULL;
CREATE INDEX idx_finance_records_sale_id          ON finance_records (sale_id)         WHERE sale_id IS NOT NULL;
CREATE INDEX idx_finance_records_service_job_id   ON finance_records (service_job_id)  WHERE service_job_id IS NOT NULL;

CREATE TRIGGER trg_finance_records_updated_at
    BEFORE UPDATE ON finance_records
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
