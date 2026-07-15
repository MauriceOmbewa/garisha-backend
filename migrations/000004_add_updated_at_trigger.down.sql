DROP TRIGGER IF EXISTS trg_users_updated_at   ON users;
DROP TRIGGER IF EXISTS trg_tenants_updated_at ON tenants;
DROP FUNCTION IF EXISTS set_updated_at();
