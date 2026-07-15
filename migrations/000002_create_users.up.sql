-- ---------------------------------------------------------------------------
-- users
-- Platform users.  Authentication is exclusively via Google Sign-In.
-- google_sub is Google's stable, unique identifier for a Google account and
-- is used to look up or create a user on every login.
--
-- Users are always scoped to a tenant.  Super-admin users have a NULL
-- tenant_id and are handled separately at the application layer.
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         REFERENCES tenants(id) ON DELETE CASCADE,

    -- Google identity
    google_sub  VARCHAR(255) NOT NULL,
    email       VARCHAR(255) NOT NULL,
    name        VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url  TEXT,

    -- RBAC
    role        VARCHAR(50)  NOT NULL DEFAULT 'customer',

    -- Lifecycle
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,

    -- Audit timestamps
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- A Google account may only exist once per tenant.
    CONSTRAINT uq_users_tenant_google UNIQUE (tenant_id, google_sub),
    -- Email must be unique within a tenant.
    CONSTRAINT uq_users_tenant_email  UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant_id  ON users (tenant_id);
CREATE INDEX idx_users_google_sub ON users (google_sub);
CREATE INDEX idx_users_email      ON users (email);
