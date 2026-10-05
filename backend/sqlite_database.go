package main

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

type SQLiteDatabase struct {
	db *sql.DB
}

func NewSQLiteDatabase(path string) (*SQLiteDatabase, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	const schema = `
	CREATE TABLE IF NOT EXISTS users (
		email TEXT PRIMARY KEY NOT NULL,
		created_at INTEGER NOT NULL,
		password_hash TEXT NOT NULL,
		last_ip TEXT,
		second_last_ip TEXT
	);
	`

	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteDatabase{db: db}, nil
}

func (database *SQLiteDatabase) Close() error {
	return database.db.Close()
}
