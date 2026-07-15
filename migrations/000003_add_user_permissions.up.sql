-- ---------------------------------------------------------------------------
-- Add per-user permission overrides to the users table.
--
-- permissions is a text[] column storing extra permissions granted to a
-- specific user beyond their role's defaults (e.g. a Mechanic who is also
-- allowed to view finance reports).  The application merges these with the
-- role's default set at authorisation time.
--
-- This column is intentionally additive — revocation of role permissions
-- is handled by assigning a different role, not by a deny list.
-- ---------------------------------------------------------------------------
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS permissions TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN users.permissions IS
    'Extra permissions granted to this user on top of their role defaults.';
