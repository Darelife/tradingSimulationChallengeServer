package main

import (
	"context"
	"database/sql"
	"time"

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
	CREATE TABLE IF NOT EXISTS registration_challenges (
    id TEXT PRIMARY KEY NOT NULL,
    email TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER
	);
	`

	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteDatabase{db: db}, nil
}

func (database *SQLiteDatabase) CreateUser(
	ctx context.Context,
	email string,
	passwordHash string,
	lastIP string,
) error {
	_, err := database.db.ExecContext(
		ctx,
		`
		INSERT INTO users (
			email,
			created_at,
			password_hash,
			last_ip
		)
		VALUES (?, ?, ?, ?)
		`,
		email,
		time.Now().Unix(),
		passwordHash,
		lastIP,
	)

	return err
}

func (database *SQLiteDatabase) UserExists(
	ctx context.Context,
	email string,
) (bool, error) {
	var exists bool

	err := database.db.QueryRowContext(
		ctx,
		`
		SELECT EXISTS(
			SELECT 1 FROM users WHERE email = ?
		)
		`,
		email,
	).Scan(&exists)

	return exists, err
}

func (database *SQLiteDatabase) SaveRegistrationChallenge(
	ctx context.Context,
	challenge RegistrationChallenge,
) error {
	_, err := database.db.ExecContext(
		ctx,
		`
		INSERT INTO registration_challenges (
			id,
			email,
			code_hash,
			expires_at
		)
		VALUES (?, ?, ?, ?)
		`,
		challenge.ID,
		challenge.Email,
		challenge.CodeHash,
		challenge.ExpiresAt.Unix(),
	)

	return err
}

func (database *SQLiteDatabase) GetRegistrationChallenge(
	ctx context.Context,
	id string,
) (RegistrationChallenge, error) {
	var challenge RegistrationChallenge
	var expiresAt int64
	var consumedAt sql.NullInt64

	err := database.db.QueryRowContext(
		ctx,
		`
		SELECT id, email, code_hash, expires_at, consumed_at
		FROM registration_challenges
		WHERE id = ?
		`,
		id,
	).Scan(
		&challenge.ID,
		&challenge.Email,
		&challenge.CodeHash,
		&expiresAt,
		&consumedAt,
	)

	if err != nil {
		return challenge, err
	}

	challenge.ExpiresAt = time.Unix(expiresAt, 0)

	if consumedAt.Valid {
		consumed := time.Unix(consumedAt.Int64, 0)
		challenge.ConsumedAt = &consumed
	}

	return challenge, nil
}

func (database *SQLiteDatabase) CompleteRegistration(
	ctx context.Context,
	challengeID string,
	passwordHash string,
	lastIP string,
) error {
	transaction, err := database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer transaction.Rollback()

	now := time.Now().Unix()

	result, err := transaction.ExecContext(
		ctx,
		`
		UPDATE registration_challenges
		SET consumed_at = ?
		WHERE id = ?
		  AND consumed_at IS NULL
		  AND expires_at > ?
		`,
		now,
		challengeID,
		now,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return sql.ErrNoRows
	}

	var email string

	err = transaction.QueryRowContext(
		ctx,
		`
		SELECT email
		FROM registration_challenges
		WHERE id = ?
		`,
		challengeID,
	).Scan(&email)
	if err != nil {
		return err
	}

	_, err = transaction.ExecContext(
		ctx,
		`
		INSERT INTO users (
			email,
			created_at,
			password_hash,
			last_ip
		)
		VALUES (?, ?, ?, ?)
		`,
		email,
		now,
		passwordHash,
		lastIP,
	)
	if err != nil {
		return err
	}

	return transaction.Commit()
}

func (database *SQLiteDatabase) Close() error {
	return database.db.Close()
}
