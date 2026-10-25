CREATE TABLE settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    public boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO settings (key, value, public) VALUES ('registration_enabled', 'true', true);
