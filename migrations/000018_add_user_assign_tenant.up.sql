-- ---------------------------------------------------------------------------
-- Allow a user's tenant_id and role to be updated after initial registration.
-- This is used when a user self-registers a new yard from the /my-yards page.
-- No schema change needed — the users table already has a nullable tenant_id.
-- This migration is intentionally empty but kept for version continuity.
-- ---------------------------------------------------------------------------
SELECT 1; -- no-op
