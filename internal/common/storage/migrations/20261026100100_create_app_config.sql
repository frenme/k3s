CREATE TABLE app_config (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    public boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO app_config (key, value, public) VALUES ('registration_enabled', 'true', true);
