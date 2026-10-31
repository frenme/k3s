CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user', 'admin')),
    PRIMARY KEY (user_id, role)
);
