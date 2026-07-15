-- ---------------------------------------------------------------------------
-- payments
-- Records every payment transaction against a hire booking, vehicle sale,
-- or service job.  M-PESA STK Push payments store the CheckoutRequestID
-- so incoming Daraja callbacks can be matched and status updated.
--
-- Status lifecycle:
--   pending → completed | failed | cancelled
-- ---------------------------------------------------------------------------
CREATE TABLE payments (
    id          UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID          NOT NULL REFERENCES tenants(id)  ON DELETE CASCADE,

    -- Source transaction (exactly one should be set)
    hire_booking_id UUID REFERENCES hire_bookings(id) ON DELETE SET NULL,
    sale_id         UUID REFERENCES vehicle_sales(id) ON DELETE SET NULL,
    service_job_id  UUID REFERENCES service_jobs(id)  ON DELETE SET NULL,

    -- Customer who is paying (optional — walk-in cash payments may omit)
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,

    -- Payment channel
    payment_method VARCHAR(30)    NOT NULL,
    -- 'mpesa' | 'cash' | 'bank_transfer' | 'card' | 'other'

    -- Amount
    amount         NUMERIC(14, 2) NOT NULL,
    currency       VARCHAR(5)     NOT NULL DEFAULT 'KES',

    -- Status lifecycle
    status         VARCHAR(20)    NOT NULL DEFAULT 'pending',
    -- 'pending' | 'completed' | 'failed' | 'cancelled'

    -- M-PESA specific (null for non-M-PESA payments)
    mpesa_phone            VARCHAR(20),   -- phone number used for STK push
    mpesa_checkout_req_id  VARCHAR(100),  -- CheckoutRequestID from STK push response
    mpesa_receipt_number   VARCHAR(50),   -- M-PESA receipt number on success
    mpesa_result_code      INTEGER,       -- Daraja ResultCode (0 = success)
    mpesa_result_desc      VARCHAR(300),  -- Daraja ResultDesc

    -- Generic reference (bank ref, receipt number for cash, etc.)
    reference   VARCHAR(200),

    -- Failure reason for non-M-PESA failures
    failure_reason VARCHAR(500),

    -- Timestamps
    paid_at     TIMESTAMPTZ,   -- set when payment completes
    created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    notes       TEXT,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_payment_method CHECK (
        payment_method IN ('mpesa','cash','bank_transfer','card','other')
    ),
    CONSTRAINT chk_payment_status CHECK (
        status IN ('pending','completed','failed','cancelled')
    ),
    CONSTRAINT chk_payment_amount CHECK (amount > 0)
);

CREATE INDEX idx_payments_tenant_id           ON payments (tenant_id);
CREATE INDEX idx_payments_hire_booking_id     ON payments (hire_booking_id)  WHERE hire_booking_id IS NOT NULL;
CREATE INDEX idx_payments_sale_id             ON payments (sale_id)          WHERE sale_id IS NOT NULL;
CREATE INDEX idx_payments_service_job_id      ON payments (service_job_id)   WHERE service_job_id IS NOT NULL;
CREATE INDEX idx_payments_customer_id         ON payments (customer_id)      WHERE customer_id IS NOT NULL;
CREATE INDEX idx_payments_status              ON payments (tenant_id, status);
CREATE INDEX idx_payments_mpesa_checkout_id   ON payments (mpesa_checkout_req_id) WHERE mpesa_checkout_req_id IS NOT NULL;

CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
