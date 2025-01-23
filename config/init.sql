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

CREATE TABLE chats (
    chat_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_name VARCHAR(255) NOT NULL,
    initiator UUID NOT NULL REFERENCES users(id),
    recipient UUID NOT NULL REFERENCES users(id),
    state VARCHAR(20) NOT NULL CHECK (state IN ('Pending', 'Active')), --Needed for chat creation?
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(chat_id),
    sender UUID NOT NULL REFERENCES users(id),
    content TEXT NOT NULL, -- TODO: Encrypted Content? Extra fields to support encryption?
    sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

