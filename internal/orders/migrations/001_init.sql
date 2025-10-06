CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    item text NOT NULL,
    amount_cents bigint NOT NULL CHECK (amount_cents >= 0),
    idempotency_key text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX orders_idempotency_key_idx ON orders (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX orders_user_id_idx ON orders (user_id, created_at DESC);
