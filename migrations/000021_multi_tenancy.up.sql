-- ---------------------------------------------------------------------------
-- Multi-tenancy: a user may belong to multiple businesses with different
-- roles in each.  The user_tenants junction table replaces the single
-- tenant_id / branch_id / role columns on the users table.
--
-- The users table keeps its tenant_id column (not dropped) to avoid breaking
-- existing data and foreign keys.  New code reads from user_tenants instead.
-- ---------------------------------------------------------------------------

CREATE TABLE user_tenants (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID         NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    tenant_id   UUID         NOT NULL REFERENCES tenants(id)  ON DELETE CASCADE,
    branch_id   UUID         REFERENCES branches(id)          ON DELETE SET NULL,
    role        VARCHAR(50)  NOT NULL DEFAULT 'customer',
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    invited_by  UUID         REFERENCES users(id)             ON DELETE SET NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- A user can only have one active membership per tenant
    CONSTRAINT uq_user_tenant UNIQUE (user_id, tenant_id)
);

CREATE INDEX idx_user_tenants_user_id   ON user_tenants (user_id);
CREATE INDEX idx_user_tenants_tenant_id ON user_tenants (tenant_id);
CREATE INDEX idx_user_tenants_branch_id ON user_tenants (branch_id);

CREATE TRIGGER trg_user_tenants_updated_at
    BEFORE UPDATE ON user_tenants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Migrate existing user-tenant relationships into user_tenants.
-- Users who already have a tenant_id get a row with their current role.
-- ---------------------------------------------------------------------------
INSERT INTO user_tenants (user_id, tenant_id, branch_id, role, is_active)
SELECT id, tenant_id, branch_id, role, is_active
FROM   users
WHERE  tenant_id IS NOT NULL
ON CONFLICT (user_id, tenant_id) DO NOTHING;
