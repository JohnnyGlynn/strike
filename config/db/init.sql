CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    user_id UUID PRIMARY KEY NOT NULL, -- If no UUID provided let postgres do it.
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    salt BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_keys (
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    encryption_public_key BYTEA NOT NULL,
    signing_public_key BYTEA NOT NULL,
    PRIMARY KEY (user_id)
);

-- Auto-update updated_at
-- CREATE OR REPLACE FUNCTION auto_update_timestamp_column()
-- RETURNS TRIGGER AS $auto_update$
-- BEGIN
--   NEW.updated_at = CURRENT_TIMESTAMP;
--   RETURN NEW;
-- END;
-- $auto_update$ LANGUAGE plpgsql;

-- CREATE TRIGGER chats_update_timestamp
-- BEFORE UPDATE ON chats
-- FOR EACH ROW
-- EXECUTE FUNCTION auto_update_timestamp_column();
