CREATE TABLE IF NOT EXISTS users (
    email TEXT PRIMARY KEY NOT NULL,
    created_at INTEGER NOT NULL,
    password_hash TEXT NOT NULL,
    last_ip TEXT,
    second_last_ip TEXT,
    is_admin INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS registration_challenges (
    id TEXT PRIMARY KEY NOT NULL,
    email TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER
);

CREATE TABLE IF NOT EXISTS password_reset_challenges (
    id TEXT PRIMARY KEY NOT NULL,
    email TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER
);

CREATE TABLE IF NOT EXISTS accounts (
    email TEXT PRIMARY KEY NOT NULL,
    balance_cents INTEGER NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY NOT NULL,
    email TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS companies (
    symbol TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    complementary_of TEXT,
    base_price_cents INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS price_points (
    symbol TEXT NOT NULL,
    ts INTEGER NOT NULL,
    price_cents INTEGER NOT NULL,
    PRIMARY KEY (symbol, ts)
);

CREATE TABLE IF NOT EXISTS holdings (
    email TEXT NOT NULL,
    symbol TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    avg_cost_cents INTEGER NOT NULL,
    PRIMARY KEY (email, symbol)
);

CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL CHECK (side IN ('buy', 'sell')),
    quantity INTEGER NOT NULL,
    price_cents INTEGER NOT NULL,
    total_cents INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
