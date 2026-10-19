package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
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
	`

	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, err
	}

	// Migrate pre-existing databases; duplicate-column errors mean the
	// column is already there.
	for _, migration := range []string{
		`ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := db.ExecContext(context.Background(), migration); err != nil &&
			!strings.Contains(err.Error(), "duplicate column name") {
			db.Close()
			return nil, err
		}
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

func (database *SQLiteDatabase) SaveChallenge(
	ctx context.Context,
	table ChallengeTable,
	challenge RegistrationChallenge,
) error {
	_, err := database.db.ExecContext(
		ctx,
		fmt.Sprintf(
			`
			INSERT INTO %s (
				id,
				email,
				code_hash,
				expires_at
			)
			VALUES (?, ?, ?, ?)
			`,
			table,
		),
		challenge.ID,
		challenge.Email,
		challenge.CodeHash,
		challenge.ExpiresAt.Unix(),
	)

	return err
}

func (database *SQLiteDatabase) GetChallenge(
	ctx context.Context,
	table ChallengeTable,
	id string,
) (RegistrationChallenge, error) {
	var challenge RegistrationChallenge
	var expiresAt int64
	var consumedAt sql.NullInt64

	err := database.db.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`
			SELECT id, email, code_hash, expires_at, consumed_at
			FROM %s
			WHERE id = ?
			`,
			table,
		),
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

// consumeChallenge atomically marks the challenge as used and returns its
// email, or sql.ErrNoRows if it is unknown, expired, or already used.
func consumeChallenge(
	ctx context.Context,
	transaction *sql.Tx,
	table ChallengeTable,
	challengeID string,
) (string, error) {
	now := time.Now().Unix()

	result, err := transaction.ExecContext(
		ctx,
		fmt.Sprintf(
			`
			UPDATE %s
			SET consumed_at = ?
			WHERE id = ?
			  AND consumed_at IS NULL
			  AND expires_at > ?
			`,
			table,
		),
		now,
		challengeID,
		now,
	)
	if err != nil {
		return "", err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	if rowsAffected != 1 {
		return "", sql.ErrNoRows
	}

	var email string

	err = transaction.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`
			SELECT email
			FROM %s
			WHERE id = ?
			`,
			table,
		),
		challengeID,
	).Scan(&email)
	if err != nil {
		return "", err
	}

	return email, nil
}

func (database *SQLiteDatabase) CompleteRegistration(
	ctx context.Context,
	challengeID string,
	passwordHash string,
	lastIP string,
	isAdmin bool,
) error {
	transaction, err := database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer transaction.Rollback()

	email, err := consumeChallenge(ctx, transaction, registrationChallenges, challengeID)
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
		time.Now().Unix(),
		passwordHash,
		lastIP,
	)
	if err != nil {
		return err
	}
	adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	adminFlag := 0
	if isAdmin || (adminEmail != "" && strings.ToLower(email) == adminEmail) {
		adminFlag = 1
	}

	_, err = transaction.ExecContext(
		ctx,
		`
		INSERT INTO accounts (
			email,
			balance_cents,
			is_admin
		)
		VALUES (?, ?, ?)
		`,
		email,
		StartingBalanceCents,
		adminFlag,
	)
	if err != nil {
		return err
	}

	return transaction.Commit()
}

func (database *SQLiteDatabase) CompletePasswordReset(
	ctx context.Context,
	challengeID string,
	passwordHash string,
) error {
	transaction, err := database.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer transaction.Rollback()

	email, err := consumeChallenge(ctx, transaction, passwordResetChallenges, challengeID)
	if err != nil {
		return err
	}

	result, err := transaction.ExecContext(
		ctx,
		`
		UPDATE users
		SET password_hash = ?
		WHERE email = ?
		`,
		passwordHash,
		email,
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

	return transaction.Commit()
}

func (database *SQLiteDatabase) Close() error {
	return database.db.Close()
}
