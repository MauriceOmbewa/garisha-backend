# Customer Management Module - Usage Guide

## Overview
The customers module provides tenant-scoped customer profile management for the Garisha platform. Customers can be linked to bookings, sales, and service jobs. They may optionally be linked to platform user accounts (for self-service portals), while walk-in customers created by staff have no login.

## Database Schema
```sql
CREATE TABLE customers (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id     UUID         REFERENCES users(id) ON DELETE SET NULL,
    full_name   VARCHAR(255) NOT NULL,
    email       VARCHAR(255),
    phone       VARCHAR(50),
    id_number   VARCHAR(100),
    id_type     VARCHAR(30),  -- 'national_id' | 'passport' | 'driving_license' | 'other'
    country     VARCHAR(100),
    city        VARCHAR(100),
    address     TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    notes       TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_customers_tenant_email UNIQUE (tenant_id, email)
);
```

## API Endpoints

### List Customers
```http
GET /api/v1/customers
Authorization: Bearer <jwt>
X-Tenant-ID: <tenant-uuid>
```
Query parameters:
- `search` (optional): Search in full_name, email, or phone
- `active` (optional): Filter by active status (true/false)

### Create Customer
```http
POST /api/v1/customers
Authorization: Bearer <jwt>
X-Tenant-ID: <tenant-uuid>

{
    "full_name": "John Doe",
    "email": "john@example.com",
    "phone": "+254712345678",
    "id_number": "12345678",
    "id_type": "national_id",
    "country": "Kenya",
    "city": "Nairobi",
    "address": "123 Main Street",
    "notes": "Preferred customer"
}
```

### Get Customer
```http
GET /api/v1/customers/{id}
Authorization: Bearer <jwt>
X-Tenant-ID: <tenant-uuid>
```

### Update Customer
```http
PATCH /api/v1/customers/{id}
Authorization: Bearer <jwt>
X-Tenant-ID: <tenant-uuid>

{
    "full_name": "John Smith",
    "is_active": false
}
```

### Delete Customer
```http
DELETE /api/v1/customers/{id}
Authorization: Bearer <jwt>
X-Tenant-ID: <tenant-uuid>
```

## Integration with Other Modules

### Hire Bookings
When creating a car hire booking, you would reference a customer:
```go
type Booking struct {
    ID         string
    TenantID   string
    VehicleID  string
    CustomerID string  // References customers.id
    StartDate  time.Time
    EndDate    time.Time
    Status     BookingStatus
    // ... other fields
}
```

### Vehicle Sales
When recording a vehicle sale, you would reference a customer as the buyer:
```go
type Sale struct {
    ID         string
    TenantID   string
    VehicleID  string
    CustomerID string  // References customers.id (buyer)
    SalePrice  float64
    SaleDate   time.Time
    // ... other fields
}
```

### Service Jobs
When creating a service job, you would reference the customer who owns/uses the vehicle:
```go
type ServiceJob struct {
    ID         string
    TenantID   string
    VehicleID  string
    CustomerID string  // References customers.id
    JobType    string
    Status     ServiceStatus
    // ... other fields
}
```

## RBAC Permissions
- `customer.view`: Required for GET endpoints
- `customer.create`: Required for POST
- `customer.update`: Required for PATCH
- `customer.delete`: Required for DELETE

## Business Rules
1. Email must be unique per tenant (nullable for walk-in customers)
2. Customers can be linked to user accounts via `user_id` field
3. Prefer deactivation (`is_active=false`) over hard deletion for customers with transaction history
4. ID type must be one of: `national_id`, `passport`, `driving_license`, `other`

## Example Service Usage
```go
// In hire/sales/service modules:
func (s *SomeService) CreateTransaction(ctx context.Context, customerID string, /* other params */) error {
    // Verify customer exists and belongs to tenant
    cust, err := customersRepo.FindByID(ctx, customerID)
    if err != nil {
        return fmt.Errorf("failed to get customer: %w", err)
    }
    if cust == nil || cust.TenantID != tenantID || !cust.IsActive {
        return apperr.BadRequest("invalid or inactive customer")
    }
    
    // Proceed with transaction creation...
}
```