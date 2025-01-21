CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- If no UUID provided let postgres do it.
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    salt BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_keys (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    encryption_public_key BYTEA NOT NULL,
    signing_public_key BYTEA NOT NULL,
    PRIMARY KEY (user_id)
);
