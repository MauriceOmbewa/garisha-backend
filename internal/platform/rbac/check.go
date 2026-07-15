package rbac

// HasWithOverrides is like Has but also checks an additional slice of
// permission strings granted directly to a user (from users.permissions).
// This is used when the full user record is available (e.g. in service
// layer checks), while the Authorize middleware uses Has with the role alone
// since the JWT claims do not carry the full permission list.
func HasWithOverrides(role Role, perm Permission, overrides []string) bool {
	if Has(role, perm) {
		return true
	}

	for _, o := range overrides {
		if Permission(o) == perm {
			return true
		}
	}

	return false
}
