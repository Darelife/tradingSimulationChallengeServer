CREATE TABLE IF NOT EXISTS users (
    email TEXT PRIMARY KEY NOT NULL,
    created_at INTEGER NOT NULL,
    password_hash TEXT NOT NULL,
    last_ip TEXT,
    second_last_ip TEXT
);

CREATE TABLE IF NOT EXISTS registration_challenges (
    id TEXT PRIMARY KEY NOT NULL,
    email TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER
);
