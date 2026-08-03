-- ---------------------------------------------------------------------------
-- Add branch_id to users so branch-scoped roles can be restricted
-- to a specific location.  NULL = not assigned to a specific branch
-- (used for owner, admin, accountant roles who see all branches).
-- ---------------------------------------------------------------------------
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;

COMMENT ON COLUMN users.branch_id IS
    'The branch this user is assigned to. NULL means cross-branch access (owner/admin/accountant).';

CREATE INDEX IF NOT EXISTS idx_users_branch_id ON users (branch_id);
