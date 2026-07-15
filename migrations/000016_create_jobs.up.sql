-- ---------------------------------------------------------------------------
-- jobs
-- Persistent async job queue backed by PostgreSQL.
-- Workers SELECT ... FOR UPDATE SKIP LOCKED to claim jobs without contention.
--
-- Lifecycle:
--   pending → running → completed | failed | dead
--
-- A job moves to 'dead' after max_attempts are exhausted.
-- ---------------------------------------------------------------------------
CREATE TABLE jobs (
    id              UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID          REFERENCES tenants(id) ON DELETE CASCADE,
    -- NULL tenant_id = system-level job (e.g. payment polling)

    -- Job classification
    type            VARCHAR(100)  NOT NULL,
    -- 'send_email' | 'send_sms' | 'poll_mpesa_payment'
    -- 'send_reminder' | 'send_notification' | 'send_webhook'

    -- Arbitrary JSON payload for the handler
    payload         JSONB         NOT NULL DEFAULT '{}',

    -- Scheduling
    run_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    -- Jobs with run_at in the future are not picked up until that time.

    -- Status lifecycle
    status          VARCHAR(20)   NOT NULL DEFAULT 'pending',
    -- 'pending' | 'running' | 'completed' | 'failed' | 'dead'

    -- Retry tracking
    attempts        INTEGER       NOT NULL DEFAULT 0,
    max_attempts    INTEGER       NOT NULL DEFAULT 3,
    last_error      TEXT,

    -- Timing
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,

    -- Deduplication key — optional; if set, only one job with this key
    -- can be in pending/running state at a time for a given type.
    idempotency_key VARCHAR(255),

    -- Who/what enqueued this job
    created_by      VARCHAR(100),  -- 'system' | user_id | service name

    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_job_status       CHECK (status IN ('pending','running','completed','failed','dead')),
    CONSTRAINT chk_job_attempts     CHECK (attempts >= 0),
    CONSTRAINT chk_job_max_attempts CHECK (max_attempts > 0),
    CONSTRAINT uq_job_idempotency   UNIQUE (type, idempotency_key)
);

CREATE INDEX idx_jobs_pickup   ON jobs (status, run_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX idx_jobs_tenant   ON jobs (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_jobs_type     ON jobs (type, status);
CREATE INDEX idx_jobs_created  ON jobs (created_at DESC);

CREATE TRIGGER trg_jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
