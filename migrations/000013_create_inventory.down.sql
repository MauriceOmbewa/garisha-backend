DROP TRIGGER IF EXISTS trg_inventory_apply_movement      ON inventory_usage;
DROP FUNCTION IF EXISTS apply_inventory_movement();

DROP TRIGGER IF EXISTS trg_inventory_items_updated_at    ON inventory_items;

DROP TABLE  IF EXISTS inventory_usage;
DROP TABLE  IF EXISTS inventory_items;
