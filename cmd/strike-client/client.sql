-- PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS addressbook (
    user_id TEXT PRIMARY KEY NOT NULL,
    username TEXT UNIQUE NOT NULL,
    enc_pkey BLOB NOT NULL,
    sig_pkey BLOB NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chats (
    chat_id TEXT PRIMARY KEY,
    chat_name TEXT NOT NULL,
    initiator TEXT NOT NULL,
    participants TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('INIT', 'KEY_EXCHANGE_PENDING', 'ENCRYPTED')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    -- FOREIGN KEY (initiator) REFERENCES addressbook(user_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    sender TEXT NOT NULL,
    receiver TEXT NOT NULL,
    direction TEXT NOT NULL,
    content BLOB NOT NULL, 
    timestamp INTEGER NOT NULL
    -- FOREIGN KEY (sender) REFERENCES addressbook(user_id)
);

