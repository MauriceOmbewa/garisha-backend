ALTER TABLE inventory_items  DROP COLUMN IF EXISTS branch_id;
ALTER TABLE payments         DROP COLUMN IF EXISTS branch_id;
ALTER TABLE finance_records  DROP COLUMN IF EXISTS branch_id;
ALTER TABLE customers        DROP COLUMN IF EXISTS branch_id;
ALTER TABLE service_jobs     DROP COLUMN IF EXISTS branch_id;
ALTER TABLE vehicle_sales    DROP COLUMN IF EXISTS branch_id;
ALTER TABLE hire_bookings    DROP COLUMN IF EXISTS branch_id;
ALTER TABLE vehicles         DROP COLUMN IF EXISTS branch_id;
DROP TABLE IF EXISTS branches;
