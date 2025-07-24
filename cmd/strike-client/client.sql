-- PRAGMA foreign_keys = ON;

CREATE TABLE identity (
    user_id TEXT PRIMARY KEY NOT NULL,
    username TEXT NOT NULL,
    enc_pkey BLOB NOT NULL,
    sig_pkey BLOB NOT NULL
  -- TODO: Include other stuff here? What happens in a multitenant system?
  -- enc_priv BLOB NOT NULL, TODO: Ephemeral users/Import PKI
  -- sig_priv BLOB NOT NULL,
  -- cert? 
);

CREATE TABLE IF NOT EXISTS addressbook (
    user_id TEXT PRIMARY KEY NOT NULL,
    username TEXT UNIQUE NOT NULL,
    enc_pkey BLOB NOT NULL,
    sig_pkey BLOB NOT NULL,
    keyex INTEGER DEFAULT 0, --key exchange state
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS friendrequests (
    friendId TEXT NOT NULL,
    username TEXT NOT NULL,
    direction TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    friendId TEXT NOT NULL, -- The friend who the chat relates too?
    direction TEXT NOT NULL,
    content BLOB NOT NULL, 
    timestamp INTEGER NOT NULL
    -- FOREIGN KEY (sender) REFERENCES addressbook(user_id)
);

