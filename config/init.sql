CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- SERVER SPECIFIC TABLES
CREATE TABLE users (
    user_id UUID PRIMARY KEY NOT NULL, -- If no UUID provided let postgres do it.
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    salt BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- CREATE TABLE user_keys (
--     user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
--     encryption_public_key BYTEA NOT NULL,
--     signing_public_key BYTEA NOT NULL,
--     PRIMARY KEY (user_id)
-- );

CREATE TABLE user_keys (
    user_id UUID NOT NULL,
    encryption_public_key BYTEA NOT NULL,
    signing_public_key BYTEA NOT NULL
);

-- CLIENT SPECIFIC TABLES
CREATE TABLE addressbook (
    user_id UUID PRIMARY KEY NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    encryption_public_key BYTEA NOT NULL,
    signing_public_key BYTEA NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chats (
    chat_id UUID PRIMARY KEY,
    chat_name VARCHAR(255) NOT NULL,
    initiator UUID NOT NULL REFERENCES addressbook(user_id),
    participants UUID[] NOT NULL,
    state VARCHAR(20) NOT NULL CHECK (state IN ('INIT', 'KEY_EXCHANGE_PENDING', 'ENCRYPTED')),
    -- TODO: Shared secret should be loaded in memeory
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chats2 (
    chat_id UUID,
    chat_name VARCHAR(255) NOT NULL,
    initiator UUID NOT NULL REFERENCES addressbook(user_id),
    participants UUID[] NOT NULL,
    state VARCHAR(20) NOT NULL CHECK (state IN ('INIT', 'KEY_EXCHANGE_PENDING', 'ENCRYPTED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(chat_id),
    sender UUID NOT NULL REFERENCES users(user_id),
    content TEXT NOT NULL, -- TODO: Encrypted Content? Extra fields to support encryption?
    sent_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages2 (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    sender UUID NOT NULL REFERENCES users(user_id),
    content TEXT NOT NULL, -- TODO: Encrypted Content? Extra fields to support encryption?
    sent_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION auto_update_timestamp_column()
RETURNS TRIGGER AS $auto_update$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$auto_update$ LANGUAGE plpgsql;

CREATE TRIGGER chats_update_timestamp
BEFORE UPDATE ON chats
FOR EACH ROW
EXECUTE FUNCTION auto_update_timestamp_column();
