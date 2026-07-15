-- ---------------------------------------------------------------------------
-- notifications
-- In-app notification records, tenant-scoped and targeted at a specific user.
-- Notifications are created by the system (e.g. booking confirmed, payment
-- received, reorder alert) and read/dismissed by the recipient user.
-- ---------------------------------------------------------------------------
CREATE TABLE notifications (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- Recipient (must be a user in this tenant)
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Classification
    type        VARCHAR(50)  NOT NULL,
    -- e.g. 'booking_confirmed' | 'payment_received' | 'service_completed'
    --      'reorder_alert' | 'sale_completed' | 'general'

    -- Human-readable content
    title       VARCHAR(255) NOT NULL,
    body        TEXT         NOT NULL,

    -- Optional deep-link reference so the client can navigate to the source
    resource_type VARCHAR(50),   -- 'hire_booking' | 'sale' | 'service_job' | 'payment' | 'inventory_item' | ...
    resource_id   UUID,          -- UUID of the related record

    -- Read state
    is_read     BOOLEAN      NOT NULL DEFAULT FALSE,
    read_at     TIMESTAMPTZ,

    -- Audit
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_tenant_user  ON notifications (tenant_id, user_id);
CREATE INDEX idx_notifications_unread       ON notifications (tenant_id, user_id) WHERE is_read = FALSE;
CREATE INDEX idx_notifications_created_at   ON notifications (tenant_id, user_id, created_at DESC);
CREATE INDEX idx_notifications_resource     ON notifications (resource_type, resource_id) WHERE resource_id IS NOT NULL;
