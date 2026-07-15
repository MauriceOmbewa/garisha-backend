-- ---------------------------------------------------------------------------
-- audit_logs
-- Immutable event log recording every significant action performed in the
-- system.  Rows are NEVER updated or deleted — this is an append-only table.
--
-- Design decisions:
--   - No FK on actor_id / resource_id: the actor or resource may be deleted
--     but the audit record must survive forever.
--   - changes JSONB stores a {"before": {...}, "after": {...}} diff so the
--     exact field values at the time of the action are preserved.
--   - ip_address and user_agent are captured for security investigations.
-- ---------------------------------------------------------------------------
CREATE TABLE audit_logs (
    id            UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id     UUID          NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Who performed the action
    actor_id      UUID,                           -- user UUID (NULL for system actions)
    actor_email   VARCHAR(255),                   -- denormalised for readability after user deletion
    actor_role    VARCHAR(50),                    -- role at time of action

    -- What was done
    action        VARCHAR(100)  NOT NULL,
    -- e.g. 'user.created', 'booking.status_changed', 'payment.completed'

    -- What it was done to
    resource_type VARCHAR(50)   NOT NULL,
    -- e.g. 'user', 'hire_booking', 'vehicle_sale', 'service_job', 'payment'
    resource_id   UUID,                           -- may be NULL for list/bulk actions

    -- Data snapshot
    changes       JSONB         NOT NULL DEFAULT '{}',
    -- {"before": {...}, "after": {...}}  or  {"data": {...}} for creates

    -- Request context
    ip_address    INET,
    user_agent    VARCHAR(500),
    request_id    VARCHAR(100),

    -- Outcome
    status        VARCHAR(10)   NOT NULL DEFAULT 'success',
    -- 'success' | 'failure'

    error_message TEXT,         -- populated when status = 'failure'

    -- When
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_audit_status CHECK (status IN ('success','failure'))
);

-- Disable updates and deletes at the database level.
CREATE RULE audit_no_update AS ON UPDATE TO audit_logs DO INSTEAD NOTHING;
CREATE RULE audit_no_delete AS ON DELETE TO audit_logs DO INSTEAD NOTHING;

CREATE INDEX idx_audit_logs_tenant_id     ON audit_logs (tenant_id);
CREATE INDEX idx_audit_logs_actor_id      ON audit_logs (actor_id)      WHERE actor_id IS NOT NULL;
CREATE INDEX idx_audit_logs_action        ON audit_logs (tenant_id, action);
CREATE INDEX idx_audit_logs_resource      ON audit_logs (tenant_id, resource_type, resource_id);
CREATE INDEX idx_audit_logs_created_at    ON audit_logs (tenant_id, created_at DESC);
