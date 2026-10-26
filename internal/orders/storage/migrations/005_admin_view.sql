CREATE VIEW admin_orders AS SELECT id, user_id, item, amount_cents, created_at FROM orders;
