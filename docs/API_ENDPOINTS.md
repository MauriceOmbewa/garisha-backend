# Garisha Backend — API Endpoints Reference

> Use this document to build your Postman collection.
> Every section maps to one Postman folder.

---

## Global Conventions

| Item | Value |
|------|-------|
| Base URL | `http://localhost:8080` |
| Content-Type | `application/json` |
| Auth header | `Authorization: Bearer <access_token>` |
| Tenant header | `X-Tenant-ID: <tenant_uuid>` *(all tenant-scoped routes)* |
| Date format | `YYYY-MM-DD` |
| Timestamp format | `YYYY-MM-DDTHH:MM:SSZ` |

### Standard Response Envelope

```json
{
  "success": true,
  "message": "human readable note",
  "data": {}
}
```

Error response:
```json
{
  "success": false,
  "message": "error description",
  "errors": [{ "field": "email", "message": "must be a valid email address" }]
}
```

---

## 1. Health

### `GET /api/v1/health`
Liveness + readiness probe. No auth required.

**Response**
```json
{
  "success": true,
  "message": "health check",
  "data": {
    "status": "ok",
    "checks": { "database": { "status": "ok" } },
    "version": "1.0.0"
  }
}
```
> Returns `503` with `"status": "degraded"` when the database is unreachable.

---

## 2. Authentication

> No `X-Tenant-ID` header required for auth endpoints.

### `POST /api/v1/auth/google`
Exchange a Google ID token for Garisha access + refresh tokens.

**Request**
```json
{
  "id_token": "eyJhbGciOi..."
}
```

**Response**
```json
{
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ..."
  }
}
```

---

### `POST /api/v1/auth/refresh`
Get a new access token using a refresh token.

**Request**
```json
{
  "refresh_token": "eyJ..."
}
```

**Response**
```json
{
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ..."
  }
}
```

---

### `GET /api/v1/auth/me`
Return the authenticated user's profile. Requires `Authorization` header only.

**Response**
```json
{
  "data": {
    "id": "uuid",
    "tenant_id": "uuid",
    "email": "user@example.com",
    "name": "Jane Doe",
    "role": "admin",
    "is_active": true
  }
}
```

---

## 3. Tenant Management *(super_admin only — no X-Tenant-ID required)*

### `GET /api/v1/admin/tenants`
List all tenants.

**Response**
```json
{
  "data": [
    { "id": "uuid", "name": "Acme Motors", "slug": "acme", "is_active": true, "created_at": "..." }
  ]
}
```

---

### `POST /api/v1/admin/tenants`
Create a new tenant.

**Request**
```json
{
  "name": "Acme Motors",
  "slug": "acme-motors",
  "email": "admin@acme.com"
}
```

**Response** `201` — created tenant object.

---

### `GET /api/v1/admin/tenants/{id}`
Get a single tenant.

---

### `PATCH /api/v1/admin/tenants/{id}`
Partially update a tenant.

**Request**
```json
{
  "name": "Acme Motors Ltd",
  "is_active": true
}
```

---

### `DELETE /api/v1/admin/tenants/{id}`
Hard-delete a tenant. Returns `204`.

---

## 4. Company Profile

### `GET /api/v1/company/profile`
Get the tenant's business profile. Requires `settings.view`.

**Response**
```json
{
  "data": {
    "id": "uuid",
    "tenant_id": "uuid",
    "legal_name": "Acme Motors Ltd",
    "business_type": "dealership",
    "registration_no": "PVT/2020/001",
    "tax_pin": "A001234567Z",
    "support_email": "support@acme.com",
    "support_phone": "+254712345678",
    "country": "Kenya",
    "city": "Nairobi",
    "currency": "KES",
    "timezone": "Africa/Nairobi",
    "enable_hire": true,
    "enable_sales": true,
    "enable_service": true,
    "social_links": { "facebook": "https://facebook.com/acme" },
    "operating_hours": {
      "monday": { "open": "08:00", "close": "17:00", "closed": false }
    }
  }
}
```

---

### `PUT /api/v1/company/profile`
Upsert the company profile. Requires `settings.update`.

**Request**
```json
{
  "legal_name": "Acme Motors Ltd",
  "business_type": "dealership",
  "registration_no": "PVT/2020/001",
  "tax_pin": "A001234567Z",
  "support_email": "support@acme.com",
  "support_phone": "+254712345678",
  "country": "Kenya",
  "city": "Nairobi",
  "address_line1": "123 Mombasa Rd",
  "postal_code": "00100",
  "currency": "KES",
  "timezone": "Africa/Nairobi",
  "enable_hire": true,
  "enable_sales": true,
  "enable_service": true,
  "primary_color": "#1E3A5F",
  "social_links": { "facebook": "https://facebook.com/acme" },
  "operating_hours": {
    "monday": { "open": "08:00", "close": "17:00", "closed": false },
    "sunday": { "open": "", "close": "", "closed": true }
  }
}
```

---

## 5. User Management

### `GET /api/v1/users`
List all users in the tenant. Requires `user.view`.

**Query params:** `?role=mechanic&is_active=true`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "email": "john@acme.com", "name": "John Doe",
      "role": "mechanic", "is_active": true, "created_at": "..."
    }
  ]
}
```

---

### `GET /api/v1/users/{id}`
Get a single user. Requires `user.view`.

---

### `DELETE /api/v1/users/{id}`
Remove a user from the tenant. Requires `user.delete`. Returns `204`.

---

### `PATCH /api/v1/users/{id}/role`
Assign a new role to a user. Requires `user.update`.

**Request**
```json
{ "role": "sales_agent" }
```
> Valid roles: `admin`, `accountant`, `mechanic`, `sales_agent`, `receptionist`, `driver`, `customer_support`, `customer`

---

### `POST /api/v1/users/{id}/activate`
Re-activate a suspended user. Requires `user.update`. No body.

---

### `POST /api/v1/users/{id}/suspend`
Suspend a user. Requires `user.update`. No body.

---

### `PUT /api/v1/users/{id}/permissions`
Set per-user permission overrides on top of their role. Requires `user.update`.

**Request**
```json
{
  "permissions": ["finance.view", "report.view"]
}
```

---

## 6. Vehicles

### `GET /api/v1/vehicles`
List all vehicles. Requires `vehicle.view`.

**Query params:** `?status=available&type=suv`

> `status`: `available` `hired` `sold` `under_service` `inactive`
> `type`: `sedan` `suv` `truck` `van` `bus` `pickup` `motorcycle` `other`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "make": "Toyota", "model": "Prado", "year": 2021,
      "color": "White", "vin": "JTEBR3FJ...", "plate_no": "KAB 123A",
      "vehicle_type": "suv", "status": "available",
      "mileage": 45000, "fuel_type": "diesel",
      "daily_rate": 8000.00, "sale_price": null,
      "images": ["https://..."],
      "notes": null, "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/vehicles`
Add a vehicle to inventory. Requires `vehicle.create`.

**Request**
```json
{
  "make": "Toyota", "model": "Prado", "year": 2021,
  "color": "White", "vin": "JTEBR3FJ...", "plate_no": "KAB 123A",
  "vehicle_type": "suv", "status": "available",
  "mileage": 45000, "fuel_type": "diesel",
  "daily_rate": 8000.00, "sale_price": 4500000.00,
  "images": ["https://cdn.example.com/v1.jpg"],
  "notes": "Full service history"
}
```

---

### `GET /api/v1/vehicles/{id}`
Get a single vehicle. Requires `vehicle.view`.

---

### `PATCH /api/v1/vehicles/{id}`
Partially update a vehicle. Requires `vehicle.update`. All fields optional.

---

### `DELETE /api/v1/vehicles/{id}`
Hard-delete a vehicle. Requires `vehicle.delete`. Returns `204`.
> Prefer setting `status: inactive` for vehicles with transaction history.

---

## 7. Customers

### `GET /api/v1/customers`
List all customers. Requires `customer.view`.

**Query params:** `?search=john&active=true`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid", "user_id": null,
      "full_name": "John Mwangi", "email": "john@email.com",
      "phone": "+254712345678", "id_number": "12345678",
      "id_type": "national_id", "country": "Kenya", "city": "Nairobi",
      "address": "123 Main St", "is_active": true,
      "notes": null, "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/customers`
Create a customer profile. Requires `customer.create`.

**Request**
```json
{
  "full_name": "John Mwangi",
  "email": "john@email.com",
  "phone": "+254712345678",
  "id_number": "12345678",
  "id_type": "national_id",
  "country": "Kenya",
  "city": "Nairobi",
  "address": "123 Main St",
  "notes": "Preferred customer"
}
```
> `id_type`: `national_id` `passport` `driving_license` `other`

---

### `GET /api/v1/customers/{id}`
Get a single customer. Requires `customer.view`.

---

### `PATCH /api/v1/customers/{id}`
Partially update a customer. Requires `customer.update`.

**Request** *(all fields optional)*
```json
{
  "full_name": "John K. Mwangi",
  "phone": "+254799999999",
  "is_active": false
}
```

---

### `DELETE /api/v1/customers/{id}`
Hard-delete a customer. Requires `customer.delete`. Returns `204`.
> Prefer `PATCH is_active: false` for customers with booking/sale history.

---

## 8. Car Hire

### `POST /api/v1/hire/availability`
Check if a vehicle is free for a date range. Requires `booking.view`.

**Request**
```json
{
  "vehicle_id": "uuid",
  "start_date": "2025-08-01",
  "end_date": "2025-08-05"
}
```

**Response**
```json
{
  "data": {
    "vehicle_id": "uuid",
    "start_date": "2025-08-01",
    "end_date": "2025-08-05",
    "available": true
  }
}
```

---

### `GET /api/v1/hire/bookings`
List bookings. Requires `booking.view`.

**Query params:** `?status=pending&vehicle_id=uuid&customer_id=uuid&from=2025-01-01&to=2025-12-31`

> `status`: `pending` `confirmed` `active` `completed` `cancelled`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "vehicle_id": "uuid", "customer_id": "uuid",
      "start_date": "2025-08-01", "end_date": "2025-08-05",
      "pickup_time": "08:00", "return_time": "17:00",
      "actual_start": null, "actual_end": null,
      "daily_rate": 8000.00, "total_days": 5,
      "total_amount": 40000.00, "deposit_amount": 10000.00,
      "discount_amount": 0.00, "final_amount": 40000.00,
      "status": "pending",
      "pickup_location": "Nairobi CBD", "return_location": "Nairobi CBD",
      "mileage_out": null, "mileage_in": null,
      "created_by": "uuid", "notes": null,
      "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/hire/bookings`
Create a hire booking. Requires `booking.create`.

**Request**
```json
{
  "vehicle_id": "uuid",
  "customer_id": "uuid",
  "start_date": "2025-08-01",
  "end_date": "2025-08-05",
  "pickup_time": "08:00",
  "return_time": "17:00",
  "daily_rate": 8000.00,
  "deposit_amount": 10000.00,
  "discount_amount": 0.00,
  "pickup_location": "Nairobi CBD",
  "return_location": "Nairobi CBD",
  "notes": "Customer prefers morning pickup"
}
```
> `total_days`, `total_amount`, `final_amount` are calculated automatically.
> Booking is created with `status: pending`.
> Returns `409` if the vehicle has a conflicting booking.

---

### `GET /api/v1/hire/bookings/{id}`
Get a single booking. Requires `booking.view`.

---

### `PATCH /api/v1/hire/bookings/{id}`
Update booking details. Requires `booking.update`. Blocked on terminal statuses.

**Request** *(all fields optional)*
```json
{
  "start_date": "2025-08-02",
  "end_date": "2025-08-06",
  "daily_rate": 7500.00,
  "discount_amount": 2000.00,
  "mileage_out": 45000,
  "notes": "Updated pickup time"
}
```

---

### `PATCH /api/v1/hire/bookings/{id}/status`
Transition booking status. Requires `booking.update`.

**Request**
```json
{ "status": "confirmed" }
```
> Valid transitions: `pending→confirmed`, `pending→cancelled`,
> `confirmed→active`, `confirmed→cancelled`,
> `active→completed`, `active→cancelled`
> `actual_start` is set automatically on `active`, `actual_end` on `completed`.

---

### `DELETE /api/v1/hire/bookings/{id}`
Hard-delete a booking. Requires `booking.delete`. Returns `204`.
> Only `pending` or `cancelled` bookings can be deleted.

---

## 9. Vehicle Sales

### `GET /api/v1/sales`
List sale records. Requires `sale.view`.

**Query params:** `?status=pending&vehicle_id=uuid&customer_id=uuid&from=2025-01-01&to=2025-12-31`

> `status`: `pending` `reserved` `completed` `cancelled`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "vehicle_id": "uuid", "customer_id": "uuid",
      "asking_price": 4500000.00, "agreed_price": 4200000.00,
      "deposit_amount": 500000.00, "discount_amount": 100000.00,
      "final_amount": 4100000.00,
      "payment_method": "bank_transfer", "payment_terms": "Balance on delivery",
      "sale_date": "2025-07-15", "handover_at": null,
      "status": "pending",
      "invoice_number": "INV-2025-001", "contract_ref": "CTR-001",
      "created_by": "uuid", "notes": null,
      "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/sales`
Record a new vehicle sale. Requires `sale.create`.

**Request**
```json
{
  "vehicle_id": "uuid",
  "customer_id": "uuid",
  "asking_price": 4500000.00,
  "agreed_price": 4200000.00,
  "deposit_amount": 500000.00,
  "discount_amount": 100000.00,
  "payment_method": "bank_transfer",
  "payment_terms": "Balance on delivery",
  "sale_date": "2025-07-15",
  "invoice_number": "INV-2025-001",
  "contract_ref": "CTR-001",
  "notes": "Trade-in included"
}
```
> `payment_method`: `cash` `mpesa` `bank_transfer` `finance` `other`
> `final_amount` = `agreed_price` − `discount_amount` (auto-calculated)
> Returns `409` if the vehicle already has an active sale.

---

### `GET /api/v1/sales/{id}`
Get a single sale. Requires `sale.view`.

---

### `PATCH /api/v1/sales/{id}`
Update sale details. Requires `sale.update`. Blocked on terminal statuses.

**Request** *(all fields optional)*
```json
{
  "agreed_price": 4100000.00,
  "deposit_amount": 600000.00,
  "payment_method": "mpesa",
  "handover_at": "2025-07-20T10:00:00Z"
}
```

---

### `PATCH /api/v1/sales/{id}/status`
Transition sale status. Requires `sale.update`.

**Request**
```json
{ "status": "reserved" }
```
> Valid transitions: `pending→reserved`, `pending→cancelled`,
> `reserved→completed`, `reserved→cancelled`
> `handover_at` is set automatically on `completed`.

---

### `DELETE /api/v1/sales/{id}`
Hard-delete a sale. Requires `sale.delete`. Returns `204`.
> Only `pending` or `cancelled` sales can be deleted.

---

## 10. Vehicle Service

### `GET /api/v1/service/jobs`
List service jobs. Requires `service.view`.

**Query params:** `?status=pending&vehicle_id=uuid&customer_id=uuid&mechanic_id=uuid&job_type=repair&from=2025-01-01T00:00:00Z&to=2025-12-31T23:59:59Z`

> `status`: `pending` `in_progress` `awaiting_parts` `completed` `cancelled`
> `job_type`: `general` `repair` `maintenance` `inspection` `bodywork` `electrical` `other`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "vehicle_id": "uuid", "customer_id": "uuid", "mechanic_id": "uuid",
      "job_type": "repair", "status": "in_progress",
      "received_at": "2025-07-01T08:00:00Z", "due_date": "2025-07-03",
      "completed_at": null, "mileage_in": 46000,
      "labour_total": 5000.00, "parts_total": 3000.00,
      "total_amount": 8000.00, "discount_amount": 500.00, "final_amount": 7500.00,
      "created_by": "uuid",
      "customer_notes": "Knocking sound from engine",
      "internal_notes": "Replace timing belt",
      "items": [
        {
          "id": "uuid", "job_id": "uuid", "item_type": "labour",
          "description": "Timing belt replacement", "quantity": 1,
          "unit_price": 5000.00, "total_price": 5000.00, "part_number": null
        }
      ],
      "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/service/jobs`
Open a service job. Requires `service.create`.

**Request**
```json
{
  "vehicle_id": "uuid",
  "customer_id": "uuid",
  "mechanic_id": "uuid",
  "job_type": "repair",
  "received_at": "2025-07-01T08:00:00Z",
  "due_date": "2025-07-03",
  "mileage_in": 46000,
  "customer_notes": "Knocking sound from engine",
  "internal_notes": "Check timing belt"
}
```

---

### `GET /api/v1/service/jobs/{id}`
Get a single job with all items. Requires `service.view`.

---

### `PATCH /api/v1/service/jobs/{id}`
Update job details. Requires `service.update`. Blocked on terminal statuses.

**Request** *(all fields optional)*
```json
{
  "mechanic_id": "uuid",
  "due_date": "2025-07-04",
  "discount_amount": 500.00,
  "customer_notes": "Updated: knocking from front axle"
}
```

---

### `PATCH /api/v1/service/jobs/{id}/status`
Transition job status. Requires `service.update`.

**Request**
```json
{ "status": "in_progress" }
```
> Valid transitions: `pending→in_progress`, `pending→cancelled`,
> `in_progress→awaiting_parts`, `in_progress→completed`, `in_progress→cancelled`,
> `awaiting_parts→in_progress`, `awaiting_parts→cancelled`
> `completed_at` is set automatically on `completed`.

---

### `PATCH /api/v1/service/jobs/{id}/mechanic`
Assign or reassign a mechanic. Requires `service.update`.

**Request**
```json
{ "mechanic_id": "uuid" }
```

---

### `DELETE /api/v1/service/jobs/{id}`
Delete a job. Requires `service.delete`. Returns `204`.
> Only `pending` or `cancelled` jobs can be deleted.

---

### `GET /api/v1/service/jobs/{id}/items`
List all line items for a job. Requires `service.view`.

---

### `POST /api/v1/service/jobs/{id}/items`
Add a labour or parts line item. Requires `service.update`.

**Request**
```json
{
  "item_type": "part",
  "description": "Timing Belt Kit",
  "quantity": 1,
  "unit_price": 3000.00,
  "part_number": "TB-4Y-001"
}
```
> `item_type`: `labour` `part` `consumable` `other`
> Job totals (`labour_total`, `parts_total`, `total_amount`, `final_amount`) update automatically.

---

### `PATCH /api/v1/service/jobs/{id}/items/{item_id}`
Update a line item. Requires `service.update`.

**Request** *(all fields optional)*
```json
{
  "quantity": 2,
  "unit_price": 2800.00
}
```

---

### `DELETE /api/v1/service/jobs/{id}/items/{item_id}`
Remove a line item. Requires `service.update`. Returns `204`.

---

## 11. Finance

### `GET /api/v1/finance/summary`
Aggregated income/expense totals. Requires `finance.view`.

**Query params:** `?from=2025-01-01&to=2025-12-31`

**Response**
```json
{
  "data": {
    "total_income": 500000.00,
    "total_expenses": 120000.00,
    "net_balance": 380000.00
  }
}
```

---

### `GET /api/v1/finance/categories`
List categories. Requires `finance.view`.

**Query params:** `?type=income`

> `type`: `income` `expense`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "name": "Hire Revenue", "type": "income",
      "description": null, "is_active": true,
      "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/finance/categories`
Create a category. Requires `finance.create`.

**Request**
```json
{
  "name": "Fuel Expenses",
  "type": "expense",
  "description": "Vehicle fuel costs"
}
```

---

### `GET /api/v1/finance/categories/{id}`
Get a single category. Requires `finance.view`.

---

### `PATCH /api/v1/finance/categories/{id}`
Update a category. Requires `finance.update`.

**Request** *(all fields optional)*
```json
{
  "name": "Fuel & Maintenance",
  "is_active": true
}
```

---

### `DELETE /api/v1/finance/categories/{id}`
Delete a category. Requires `finance.update`. Returns `204`.
> Returns `409` if the category has associated records.

---

### `GET /api/v1/finance/records`
List finance records. Requires `finance.view`.

**Query params:** `?type=income&category_id=uuid&from=2025-01-01&to=2025-12-31&payment_method=mpesa&hire_booking_id=uuid&sale_id=uuid&service_job_id=uuid`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid", "category_id": "uuid",
      "type": "income", "amount": 40000.00,
      "hire_booking_id": "uuid", "sale_id": null, "service_job_id": null,
      "description": "Hire payment from John Mwangi",
      "transaction_date": "2025-07-16",
      "payment_method": "mpesa", "reference": "QHJ7K2L3M4",
      "created_by": "uuid", "notes": null,
      "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/finance/records`
Create a finance record. Requires `finance.create`.

**Request**
```json
{
  "category_id": "uuid",
  "type": "income",
  "amount": 40000.00,
  "description": "Hire payment from John Mwangi",
  "transaction_date": "2025-07-16",
  "payment_method": "mpesa",
  "reference": "QHJ7K2L3M4",
  "hire_booking_id": "uuid",
  "notes": "Week 1 of 2"
}
```
> `payment_method`: `cash` `mpesa` `bank_transfer` `card` `other`
> Category `type` must match record `type` or `400` is returned.

---

### `GET /api/v1/finance/records/{id}`
Get a single record. Requires `finance.view`.

---

### `PATCH /api/v1/finance/records/{id}`
Update a record. Requires `finance.update`.

---

### `DELETE /api/v1/finance/records/{id}`
Delete a record. Requires `finance.update`. Returns `204`.

---

## 12. Payments

### `GET /api/v1/payments`
List payments. Requires `payment.view`.

**Query params:** `?status=pending&method=mpesa&customer_id=uuid&hire_booking_id=uuid&sale_id=uuid&service_job_id=uuid&from=2025-01-01&to=2025-12-31`

> `status`: `pending` `completed` `failed` `cancelled`
> `method`: `mpesa` `cash` `bank_transfer` `card` `other`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "hire_booking_id": "uuid", "sale_id": null, "service_job_id": null,
      "customer_id": "uuid",
      "payment_method": "mpesa", "amount": 40000.00, "currency": "KES",
      "status": "completed",
      "mpesa_phone": "254712345678",
      "mpesa_checkout_req_id": "ws_CO_...",
      "mpesa_receipt_number": "QHJ7K2L3M4",
      "mpesa_result_code": 0, "mpesa_result_desc": "The service request is processed successfully",
      "reference": null, "failure_reason": null,
      "paid_at": "2025-07-16T10:30:00Z",
      "created_by": null, "notes": null,
      "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `GET /api/v1/payments/{id}`
Get a single payment. Requires `payment.view`.

---

### `POST /api/v1/payments/manual`
Record a cash, card, or bank transfer payment as immediately completed. Requires `payment.create`.

**Request**
```json
{
  "hire_booking_id": "uuid",
  "customer_id": "uuid",
  "payment_method": "cash",
  "amount": 40000.00,
  "currency": "KES",
  "reference": "RCP-2025-001",
  "notes": "Cash collected at counter"
}
```
> Provide exactly one of: `hire_booking_id`, `sale_id`, `service_job_id`.

---

### `POST /api/v1/payments/mpesa`
Initiate an M-PESA STK Push. Returns a `pending` payment record. Requires `payment.create`.

**Request**
```json
{
  "hire_booking_id": "uuid",
  "customer_id": "uuid",
  "phone_number": "254712345678",
  "amount": 40000.00,
  "account_reference": "BK-2025-001",
  "description": "Hire payment",
  "notes": null
}
```
> `phone_number` must be in international format without `+` (e.g. `254712345678`).
> `account_reference` max 12 characters. `description` max 13 characters.
> Payment status moves to `completed` or `failed` when Safaricom fires the callback.

---

### `POST /api/v1/payments/mpesa/callback`
**Public endpoint — no auth required.**
Safaricom Daraja posts the STK Push result here. Do not call this manually.

---

### `PATCH /api/v1/payments/{id}/cancel`
Cancel a pending payment. Requires `payment.create`.
> Returns `400` if payment is already in a terminal state.

---

## 13. Inventory

### `GET /api/v1/inventory/items`
List stock items. Requires `inventory.view`.

**Query params:** `?category=Engine&is_active=true&needs_reorder=true&search=timing`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "name": "Timing Belt Kit", "sku": "TB-4Y-001",
      "description": "OEM timing belt kit for 4Y engine",
      "category": "Engine", "unit": "piece",
      "quantity": 4, "reorder_level": 2, "reorder_qty": 10,
      "needs_reorder": false,
      "unit_cost": 2800.00, "selling_price": 3500.00,
      "is_active": true,
      "supplier_name": "Auto Parts Ltd", "supplier_phone": "+254711000000",
      "supplier_email": "sales@autoparts.co.ke",
      "notes": null, "created_at": "...", "updated_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/inventory/items`
Add a new stock item. Requires `inventory.create`.

**Request**
```json
{
  "name": "Timing Belt Kit",
  "sku": "TB-4Y-001",
  "description": "OEM timing belt kit for 4Y engine",
  "category": "Engine",
  "unit": "piece",
  "quantity": 10,
  "reorder_level": 2,
  "reorder_qty": 10,
  "unit_cost": 2800.00,
  "selling_price": 3500.00,
  "supplier_name": "Auto Parts Ltd",
  "supplier_phone": "+254711000000",
  "supplier_email": "sales@autoparts.co.ke"
}
```
> `unit`: `piece` `litre` `kg` `metre` `set` `box` `other`
> Opening `quantity` is recorded as a receipt movement automatically.

---

### `GET /api/v1/inventory/items/{id}`
Get a single item. Requires `inventory.view`.

---

### `PATCH /api/v1/inventory/items/{id}`
Update item details. Requires `inventory.update`.
> `quantity` cannot be set directly — use adjust or usage endpoints.

---

### `DELETE /api/v1/inventory/items/{id}`
Delete an item. Requires `inventory.delete`. Returns `204`.
> Returns `409` if the item has usage history. Deactivate instead.

---

### `GET /api/v1/inventory/reorder-alerts`
List active items at or below their reorder level. Requires `inventory.view`.

---

### `GET /api/v1/inventory/usage`
List stock movement records. Requires `inventory.view`.

**Query params:** `?item_id=uuid&movement=usage&service_job_id=uuid&from=2025-01-01&to=2025-12-31`

> `movement`: `usage` `adjustment` `receipt`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid", "item_id": "uuid",
      "movement": "usage", "quantity": -1,
      "service_job_id": "uuid", "service_job_item_id": "uuid",
      "unit_cost": 2800.00, "reference": null, "notes": null,
      "created_by": "uuid", "created_at": "..."
    }
  ]
}
```

---

### `POST /api/v1/inventory/usage`
Record stock consumed in a service job. Requires `inventory.create`.

**Request**
```json
{
  "item_id": "uuid",
  "quantity": 1,
  "service_job_id": "uuid",
  "service_job_item_id": "uuid",
  "unit_cost": 2800.00,
  "notes": "Used in timing belt replacement"
}
```
> Returns `400` if stock is insufficient.

---

### `POST /api/v1/inventory/items/{id}/adjust`
Manual stock adjustment or supplier receipt. Requires `inventory.update`.

**Request**
```json
{
  "movement": "receipt",
  "quantity": 10,
  "unit_cost": 2750.00,
  "reference": "PO-2025-042",
  "notes": "Restocked from Auto Parts Ltd"
}
```
> `movement`: `adjustment` or `receipt` only (use `/usage` for consumption)
> For `receipt`, quantity must be positive.
> For `adjustment`, quantity can be negative (write-off) or positive (correction).

---

## 14. Notifications

> All notification endpoints are scoped to the **authenticated user** — users can only see and manage their own notifications.

### `GET /api/v1/notifications`
List the caller's notifications. Requires `notification.view`.

**Query params:** `?is_read=false&type=reorder_alert&limit=50&offset=0`

> `type` examples: `booking_confirmed` `booking_cancelled` `payment_received` `payment_failed` `sale_completed` `service_completed` `reorder_alert` `general`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid", "user_id": "uuid",
      "type": "payment_received",
      "title": "Payment Received",
      "body": "KES 40,000 received for booking BK-2025-001",
      "resource_type": "hire_booking", "resource_id": "uuid",
      "is_read": false, "read_at": null,
      "created_at": "..."
    }
  ]
}
```

---

### `GET /api/v1/notifications/unread-count`
Get the caller's unread notification count. Requires `notification.view`.

**Response**
```json
{ "data": { "count": 5 } }
```

---

### `PATCH /api/v1/notifications/read-all`
Mark all of the caller's unread notifications as read. Requires `notification.view`.

**Response**
```json
{ "data": { "updated": 5 } }
```

---

### `PATCH /api/v1/notifications/{id}/read`
Mark a single notification as read. Requires `notification.view`.
> Idempotent — calling on an already-read notification returns `200`.

**Response** — full notification object with `is_read: true`.

---

### `DELETE /api/v1/notifications/read`
Delete all of the caller's read notifications (housekeeping). Requires `notification.view`.

**Response**
```json
{ "data": { "deleted": 12 } }
```

---

### `DELETE /api/v1/notifications/{id}`
Delete a single notification. Requires `notification.view`. Returns `204`.

---

## 15. Audit Logs

> Read-only. Restricted to users with `audit.view` permission (`admin` and `super_admin` roles only).

### `GET /api/v1/audit/logs`
List audit log entries for the tenant. Requires `audit.view`.

**Query params:** `?actor_id=uuid&action=hire_booking.created&resource_type=hire_booking&resource_id=uuid&status=success&from=2025-01-01&to=2025-12-31&limit=50&offset=0`

> `status`: `success` `failure`
> `action` examples: `user.login` `hire_booking.status_changed` `payment.completed` `vehicle.created`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid",
      "actor_id": "uuid", "actor_email": "admin@acme.com", "actor_role": "admin",
      "action": "hire_booking.created",
      "resource_type": "hire_booking", "resource_id": "uuid",
      "changes": {
        "data": {
          "vehicle_id": "uuid", "customer_id": "uuid",
          "start_date": "2025-08-01", "status": "pending"
        }
      },
      "ip_address": "197.232.1.1",
      "user_agent": "Mozilla/5.0 ...",
      "request_id": "abc123",
      "status": "success", "error_message": null,
      "created_at": "..."
    }
  ]
}
```

---

### `GET /api/v1/audit/logs/{id}`
Get a single audit log entry. Requires `audit.view`.

---

## 16. Reports

> All report endpoints accept optional `?from=YYYY-MM-DD&to=YYYY-MM-DD` query params. Omit to get all-time data. Requires `report.view`.

### `GET /api/v1/reports/hire`
Hire bookings summary.

**Response**
```json
{
  "data": {
    "total_bookings": 142,
    "by_status": [
      { "status": "completed", "count": 98 },
      { "status": "cancelled", "count": 12 }
    ],
    "total_revenue": 1260000.00,
    "avg_duration_days": 3.5,
    "top_vehicles": [
      { "vehicle_id": "uuid", "label": "Toyota Prado 2021", "count": 24 }
    ]
  }
}
```

---

### `GET /api/v1/reports/sales`
Vehicle sales summary.

**Response**
```json
{
  "data": {
    "total_sales": 18,
    "by_status": [{ "status": "completed", "count": 15 }],
    "total_revenue": 63000000.00,
    "avg_agreed_price": 4200000.00,
    "top_vehicles": [
      { "vehicle_id": "uuid", "label": "Toyota Hilux 2022", "count": 3 }
    ]
  }
}
```

---

### `GET /api/v1/reports/service`
Service jobs summary.

**Response**
```json
{
  "data": {
    "total_jobs": 87,
    "by_status": [{ "status": "completed", "count": 72 }],
    "by_job_type": [
      { "type": "repair", "count": 34 },
      { "type": "maintenance", "count": 28 }
    ],
    "total_revenue": 435000.00,
    "avg_job_value": 6041.67,
    "top_mechanics": [
      { "mechanic_id": "uuid", "name": "James Kamau", "job_count": 22 }
    ]
  }
}
```

---

### `GET /api/v1/reports/finance`
Finance ledger summary with monthly trend.

**Response**
```json
{
  "data": {
    "total_income": 1850000.00,
    "total_expenses": 340000.00,
    "net_balance": 1510000.00,
    "by_category": [
      { "category_id": "uuid", "category_name": "Hire Revenue", "type": "income", "total": 1260000.00 }
    ],
    "by_month": [
      { "month": "2025-06", "income": 280000.00, "expense": 52000.00, "net": 228000.00 },
      { "month": "2025-07", "income": 310000.00, "expense": 61000.00, "net": 249000.00 }
    ]
  }
}
```

---

### `GET /api/v1/reports/payments`
Payments summary.

**Response**
```json
{
  "data": {
    "total_collected": 1420000.00,
    "by_method": [
      { "method": "mpesa", "total": 890000.00, "count": 64 },
      { "method": "cash",  "total": 530000.00, "count": 38 }
    ],
    "by_status": [
      { "status": "completed", "count": 102 },
      { "status": "failed", "count": 8 }
    ],
    "mpesa_success_count": 61,
    "mpesa_failure_count": 8
  }
}
```

---

## 17. Dashboard / Analytics

> All dashboard endpoints require `report.view`. No query params needed — data covers the current calendar month and last 30 days.

### `GET /api/v1/dashboard/summary`
All KPI card values for the current calendar month.

**Response**
```json
{
  "data": {
    "total_vehicles": 42,
    "available_vehicles": 28,
    "total_customers": 318,
    "active_customers": 301,
    "active_bookings": 6,
    "bookings_this_month": 24,
    "hire_revenue_month": 192000.00,
    "sales_this_month": 3,
    "sales_revenue_month": 12600000.00,
    "open_service_jobs": 4,
    "service_revenue_month": 68000.00,
    "income_this_month": 312000.00,
    "expense_this_month": 58000.00,
    "net_balance_month": 254000.00,
    "pending_payments": 3,
    "collected_this_month": 289000.00,
    "low_stock_items": 2,
    "unread_notifications": 5,
    "period_start": "2025-07-01T00:00:00Z",
    "period_end": "2025-07-31T23:59:59Z"
  }
}
```

---

### `GET /api/v1/dashboard/charts`
Time-series and distribution datasets for charts (last 30 days).

**Response**
```json
{
  "data": {
    "revenue_trend": [
      { "date": "2025-06-17", "hire": 8000.00, "sales": 0, "service": 3500.00, "total": 11500.00 }
    ],
    "booking_trend": [
      { "date": "2025-06-17", "count": 2 }
    ],
    "vehicle_status_distribution": [
      { "status": "available", "count": 28, "percent": 66.67 },
      { "status": "hired",     "count": 6,  "percent": 14.29 }
    ],
    "payment_method_distribution": [
      { "method": "mpesa", "count": 18, "total": 144000.00, "percent": 72.50 },
      { "method": "cash",  "count": 7,  "total": 54500.00,  "percent": 27.50 }
    ],
    "monthly_finance": [
      { "month": "2025-02", "income": 180000.00, "expense": 42000.00, "net": 138000.00 },
      { "month": "2025-07", "income": 312000.00, "expense": 58000.00, "net": 254000.00 }
    ]
  }
}
```

---

### `GET /api/v1/dashboard/activity`
The 20 most recent transactions across all modules.

**Response**
```json
{
  "data": [
    {
      "id": "uuid",
      "type": "booking",
      "description": "Hire booking confirmed",
      "amount": 40000.00,
      "status": "confirmed",
      "resource_id": "uuid",
      "created_at": "2025-07-16T09:15:00Z"
    },
    {
      "id": "uuid",
      "type": "payment",
      "description": "Payment via mpesa completed",
      "amount": 40000.00,
      "status": "completed",
      "resource_id": "uuid",
      "created_at": "2025-07-16T09:18:00Z"
    }
  ]
}
```

---

## 18. File Management

> Upload flow: `POST /presign` → client PUTs directly to S3 → `POST /confirm` to register in DB.
> File bytes never pass through the API server.

### `GET /api/v1/files`
List uploaded files. Requires `vehicle.view`.

**Query params:** `?resource_type=vehicle&resource_id=uuid&is_active=true`

> `resource_type`: `vehicle` `customer` `service_job` `company` `general`

**Response**
```json
{
  "data": [
    {
      "id": "uuid", "tenant_id": "uuid", "uploaded_by": "uuid",
      "storage_key": "tenants/uuid/vehicle/uuid/1721000000000_prado.jpg",
      "original_name": "prado.jpg",
      "mime_type": "image/jpeg", "size_bytes": 245760,
      "resource_type": "vehicle", "resource_id": "uuid",
      "is_active": true, "created_at": "..."
    }
  ]
}
```

---

### `GET /api/v1/files/{id}`
Get a single file record. Requires `vehicle.view`.

---

### `GET /api/v1/files/{id}/url`
Generate a download URL for a file. Requires `vehicle.view`.

**Response**
```json
{
  "data": {
    "url": "https://s3.amazonaws.com/bucket/tenants/...?X-Amz-Signature=...",
    "expires_in": 3600
  }
}
```
> `expires_in` is `0` when the bucket has a public CDN URL configured.

---

### `POST /api/v1/files/presign`
Generate a presigned S3 PUT URL for direct upload. Requires `vehicle.create`.

**Request**
```json
{
  "original_name": "prado_front.jpg",
  "mime_type": "image/jpeg",
  "size_bytes": 245760,
  "resource_type": "vehicle",
  "resource_id": "uuid"
}
```

> Accepted MIME types: `image/jpeg` `image/png` `image/webp` `image/gif`
> `application/pdf` `application/msword`
> `application/vnd.openxmlformats-officedocument.wordprocessingml.document`
> `text/csv`
> Maximum file size: **20 MB**

**Response**
```json
{
  "data": {
    "upload_url": "https://s3.amazonaws.com/bucket/tenants/...?X-Amz-Signature=...",
    "storage_key": "tenants/uuid/vehicle/uuid/1721000000000_prado_front.jpg",
    "expires_in": 900
  }
}
```
> `upload_url` is a presigned PUT URL. Set `Content-Type` to the same MIME type when uploading.

---

### `POST /api/v1/files/confirm`
Register a completed upload in the database. Call after a successful PUT to S3. Requires `vehicle.create`.

**Request**
```json
{
  "storage_key": "tenants/uuid/vehicle/uuid/1721000000000_prado_front.jpg",
  "original_name": "prado_front.jpg",
  "mime_type": "image/jpeg",
  "size_bytes": 245760,
  "resource_type": "vehicle",
  "resource_id": "uuid"
}
```
> Idempotent — calling again with the same `storage_key` returns the existing record.

**Response** `201` — full file upload record.

---

### `DELETE /api/v1/files/{id}`
Delete the object from S3 and remove the DB record. Requires `vehicle.delete`. Returns `204`.
> S3 deletion errors are logged but do not block the DB cleanup.

---

---

## Appendix A — RBAC Roles & Permissions

| Role | Key Permissions |
|------|----------------|
| `super_admin` | All permissions — no X-Tenant-ID required |
| `admin` | All tenant-level permissions |
| `accountant` | `finance.*` `payment.*` `report.*` `customer.view` |
| `mechanic` | `service.*` `vehicle.view` `inventory.view` |
| `sales_agent` | `sale.*` `customer.*` `payment.*` `vehicle.view` |
| `receptionist` | `booking.*` `customer.*` `vehicle.view` |
| `driver` | `booking.view` `vehicle.view` |
| `customer_support` | `vehicle.view` `booking.view` `customer.view` `sale.view` `service.view` |
| `customer` | `vehicle.view` `booking.create/view` `sale.view` `payment.*` |

---

## Appendix B — Status Lifecycles

### Hire Booking
```
pending → confirmed → active → completed
        ↘ cancelled    ↘ cancelled  ↘ cancelled
```

### Vehicle Sale
```
pending → reserved → completed
        ↘ cancelled  ↘ cancelled
```

### Service Job
```
pending → in_progress → awaiting_parts → (back to in_progress)
        ↘ cancelled      → completed
                         ↘ cancelled
```

### Payment
```
pending → completed
        → failed
        → cancelled
```

---

## Appendix C — Common HTTP Status Codes

| Code | Meaning |
|------|---------|
| `200` | Success |
| `201` | Resource created |
| `204` | Success, no body (delete) |
| `400` | Bad request / validation error |
| `401` | Missing or invalid token |
| `403` | Insufficient permissions |
| `404` | Resource not found |
| `409` | Conflict (duplicate, invalid state transition) |
| `422` | Validation failed (field errors in `errors` array) |
| `500` | Internal server error |
| `503` | Service unavailable (health check failed) |

---

## Appendix D — Postman Environment Variables

Set these as environment variables in Postman:

| Variable | Example Value |
|----------|--------------|
| `base_url` | `http://localhost:8080` |
| `access_token` | *(set from `/auth/google` response)* |
| `refresh_token` | *(set from `/auth/google` response)* |
| `tenant_id` | *(your tenant UUID)* |
| `vehicle_id` | *(set after creating a vehicle)* |
| `customer_id` | *(set after creating a customer)* |
| `booking_id` | *(set after creating a booking)* |
| `sale_id` | *(set after creating a sale)* |
| `job_id` | *(set after creating a service job)* |

Use Postman **Pre-request Scripts** or **Tests** to auto-capture tokens:
```javascript
// In the Tests tab of POST /api/v1/auth/google
const data = pm.response.json().data;
pm.environment.set("access_token",  data.access_token);
pm.environment.set("refresh_token", data.refresh_token);
```

Set the `Authorization` header at **collection level** as:
```
Bearer {{access_token}}
```
And `X-Tenant-ID` header at collection level as:
```
{{tenant_id}}
```
This way every request in the collection inherits both headers automatically.
