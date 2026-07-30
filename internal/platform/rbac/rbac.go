// Package rbac defines the platform's role and permission model.
//
// # Design
//
// Roles are coarse groupings of capability (e.g. Admin, Mechanic).
// Permissions are fine-grained action strings (e.g. "vehicle.create").
// A role maps to a set of permissions. Middleware reads the role from the
// JWT claims and resolves the permission set at request time — no DB query.
//
// This is the authoritative list of roles and permissions for the platform.
// When a new feature adds new actions, add the permission constant here and
// grant it to the appropriate roles in defaultPermissions.
package rbac

// ─── Roles ───────────────────────────────────────────────────────────────────

// Role is a string identifier assigned to every platform user.
type Role string

const (
	RoleSuperAdmin      Role = "super_admin"
	RoleOwner           Role = "owner"         // self-registered yard owner — full admin access
	RoleAdmin           Role = "admin"
	RoleAccountant      Role = "accountant"
	RoleMechanic        Role = "mechanic"
	RoleSalesAgent      Role = "sales_agent"
	RoleReceptionist    Role = "receptionist"
	RoleDriver          Role = "driver"
	RoleCustomerSupport Role = "customer_support"
	RoleCustomer        Role = "customer"
)

// All returns every defined role. Useful for validation.
func All() []Role {
	return []Role{
		RoleSuperAdmin,
		RoleOwner,
		RoleAdmin,
		RoleAccountant,
		RoleMechanic,
		RoleSalesAgent,
		RoleReceptionist,
		RoleDriver,
		RoleCustomerSupport,
		RoleCustomer,
	}
}

// ─── Permissions ─────────────────────────────────────────────────────────────

// Permission is a fine-grained action string checked by the Authorize middleware.
// Format: "<resource>.<action>"
type Permission string

const (
	// Tenant management (super admin only)
	PermTenantCreate Permission = "tenant.create"
	PermTenantUpdate Permission = "tenant.update"
	PermTenantDelete Permission = "tenant.delete"
	PermTenantView   Permission = "tenant.view"

	// User management
	PermUserCreate Permission = "user.create"
	PermUserUpdate Permission = "user.update"
	PermUserDelete Permission = "user.delete"
	PermUserView   Permission = "user.view"

	// Vehicle management
	PermVehicleCreate Permission = "vehicle.create"
	PermVehicleUpdate Permission = "vehicle.update"
	PermVehicleDelete Permission = "vehicle.delete"
	PermVehicleView   Permission = "vehicle.view"

	// Car hire / bookings
	PermBookingCreate Permission = "booking.create"
	PermBookingUpdate Permission = "booking.update"
	PermBookingDelete Permission = "booking.delete"
	PermBookingView   Permission = "booking.view"

	// Vehicle sales
	PermSaleCreate Permission = "sale.create"
	PermSaleUpdate Permission = "sale.update"
	PermSaleDelete Permission = "sale.delete"
	PermSaleView   Permission = "sale.view"

	// Service / repair jobs
	PermServiceCreate Permission = "service.create"
	PermServiceUpdate Permission = "service.update"
	PermServiceDelete Permission = "service.delete"
	PermServiceView   Permission = "service.view"

	// Inventory
	PermInventoryCreate Permission = "inventory.create"
	PermInventoryUpdate Permission = "inventory.update"
	PermInventoryDelete Permission = "inventory.delete"
	PermInventoryView   Permission = "inventory.view"

	// Finance
	PermFinanceCreate Permission = "finance.create"
	PermFinanceUpdate Permission = "finance.update"
	PermFinanceView   Permission = "finance.view"

	// Payments
	PermPaymentCreate Permission = "payment.create"
	PermPaymentView   Permission = "payment.view"

	// Reports
	PermReportView   Permission = "report.view"
	PermReportExport Permission = "report.export"

	// Settings
	PermSettingsView   Permission = "settings.view"
	PermSettingsUpdate Permission = "settings.update"

	// Customers
	PermCustomerCreate Permission = "customer.create"
	PermCustomerUpdate Permission = "customer.update"
	PermCustomerDelete Permission = "customer.delete"
	PermCustomerView   Permission = "customer.view"

	// Audit logs
	PermAuditView Permission = "audit.view"

	// Notifications
	PermNotificationView Permission = "notification.view"
)

// ─── Role → Permission mapping ────────────────────────────────────────────────

// permissionSet is an unordered set of permissions stored as a map for O(1)
// lookup.
type permissionSet map[Permission]struct{}

func newSet(perms ...Permission) permissionSet {
	s := make(permissionSet, len(perms))
	for _, p := range perms {
		s[p] = struct{}{}
	}
	return s
}

// defaultPermissions maps every role to its granted permissions.
// Super admin implicitly has all permissions — that is handled in Has().
var defaultPermissions = map[Role]permissionSet{
	RoleAdmin: newSet(
		PermUserCreate, PermUserUpdate, PermUserDelete, PermUserView,
		PermVehicleCreate, PermVehicleUpdate, PermVehicleDelete, PermVehicleView,
		PermBookingCreate, PermBookingUpdate, PermBookingDelete, PermBookingView,
		PermSaleCreate, PermSaleUpdate, PermSaleDelete, PermSaleView,
		PermServiceCreate, PermServiceUpdate, PermServiceDelete, PermServiceView,
		PermInventoryCreate, PermInventoryUpdate, PermInventoryDelete, PermInventoryView,
		PermFinanceCreate, PermFinanceUpdate, PermFinanceView,
		PermPaymentCreate, PermPaymentView,
		PermReportView, PermReportExport,
		PermSettingsView, PermSettingsUpdate,
		PermCustomerCreate, PermCustomerUpdate, PermCustomerDelete, PermCustomerView,
		PermAuditView,
		PermNotificationView,
	),

	RoleAccountant: newSet(
		PermVehicleView,
		PermFinanceCreate, PermFinanceUpdate, PermFinanceView,
		PermPaymentCreate, PermPaymentView,
		PermReportView, PermReportExport,
		PermCustomerView,
		PermNotificationView,
	),

	RoleMechanic: newSet(
		PermVehicleView,
		PermServiceCreate, PermServiceUpdate, PermServiceView,
		PermInventoryView,
		PermNotificationView,
	),

	RoleSalesAgent: newSet(
		PermVehicleView,
		PermSaleCreate, PermSaleUpdate, PermSaleView,
		PermCustomerCreate, PermCustomerUpdate, PermCustomerView,
		PermPaymentCreate, PermPaymentView,
		PermNotificationView,
	),

	RoleReceptionist: newSet(
		PermVehicleView,
		PermBookingCreate, PermBookingUpdate, PermBookingView,
		PermCustomerCreate, PermCustomerUpdate, PermCustomerView,
		PermNotificationView,
	),

	RoleDriver: newSet(
		PermVehicleView,
		PermBookingView,
		PermNotificationView,
	),

	RoleCustomerSupport: newSet(
		PermVehicleView,
		PermBookingView,
		PermCustomerView,
		PermSaleView,
		PermServiceView,
		PermNotificationView,
	),

	RoleCustomer: newSet(
		PermVehicleView,
		PermBookingCreate, PermBookingView,
		PermSaleView,
		PermPaymentCreate, PermPaymentView,
		PermNotificationView,
	),
}

// ─── Public API ──────────────────────────────────────────────────────────────

// Has reports whether role is granted permission.
// Super admin and owner are always granted all permissions.
func Has(role Role, perm Permission) bool {
	if role == RoleSuperAdmin || role == RoleOwner {
		return true
	}

	perms, ok := defaultPermissions[role]
	if !ok {
		return false
	}

	_, granted := perms[perm]
	return granted
}

// PermissionsFor returns all permissions granted to role as a slice.
// Useful for building "what can I do?" API responses.
func PermissionsFor(role Role) []Permission {
	if role == RoleSuperAdmin {
		// Collect every defined permission.
		seen := make(map[Permission]struct{})
		for _, set := range defaultPermissions {
			for p := range set {
				seen[p] = struct{}{}
			}
		}
		all := make([]Permission, 0, len(seen))
		for p := range seen {
			all = append(all, p)
		}
		return all
	}

	perms, ok := defaultPermissions[role]
	if !ok {
		return nil
	}

	out := make([]Permission, 0, len(perms))
	for p := range perms {
		out = append(out, p)
	}
	return out
}

// IsValidRole reports whether r is one of the platform's defined roles.
func IsValidRole(r string) bool {
	for _, role := range All() {
		if string(role) == r {
			return true
		}
	}
	return false
}
