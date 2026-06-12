-- Existing rows get an empty password_hash, which can never match a bcrypt
-- comparison, so pre-auth accounts simply cannot log in until a password is
-- set. Roles: 'user' (default) and 'admin'.
ALTER TABLE users
    ADD COLUMN password_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin'));
