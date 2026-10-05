CREATE TABLE IF NOT EXISTS users (
    email TEXT PRIMARY KEY NOT NULL,
    created_at INTEGER NOT NULL,
    password_hash TEXT NOT NULL,
    last_ip TEXT,
    second_last_ip TEXT
);
