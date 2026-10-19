package main

import (
	"context"
	"time"
)

// ChallengeTable names the SQL table backing a challenge flow.
const (
	registrationChallenges  ChallengeTable = "registration_challenges"
	passwordResetChallenges ChallengeTable = "password_reset_challenges"
)

type RegistrationChallenge struct {
	ID         string
	Email      string
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

type Database interface {
	CreateUser(
		ctx context.Context,
		email string,
		passwordHash string,
		lastIP string,
	) error

	UserExists(
		ctx context.Context,
		email string,
	) (bool, error)

	SaveChallenge(
		context.Context,
		ChallengeTable,
		RegistrationChallenge,
	) error

	GetChallenge(
		context.Context,
		ChallengeTable,
		string,
	) (RegistrationChallenge, error)

	CompleteRegistration(
		context.Context,
		string,
		string,
		string,
		bool,
	) error

	CompletePasswordReset(
		context.Context,
		string,
		string,
	) error

	// GetUserCredentials returns the user's password hash and admin flag;
	// sql.ErrNoRows if the user does not exist. (auth_db.go)
	GetUserCredentials(
		ctx context.Context,
		email string,
	) (passwordHash string, isAdmin bool, err error)



	// Market. (market_db.go)
	ListCompanies(ctx context.Context) ([]Company, error)
	SavePricePoint(ctx context.Context, point PricePoint) error
	SavePricePoints(ctx context.Context, points []PricePoint) error
	LatestPrices(ctx context.Context) (map[string]int64, error)
	PriceHistory(ctx context.Context, symbol string, since time.Time) ([]PricePoint, error)

	// Trading. ExecuteTrade atomically applies a trade at priceCents and
	// returns the executed order, or one of the Err* sentinels in types.go.
	// (trading_db.go)
	GetAccount(ctx context.Context, email string) (Account, error)
	ExecuteTrade(
		ctx context.Context,
		email string,
		symbol string,
		side string,
		quantity int64,
		priceCents int64,
	) (Order, error)
	ListOrders(ctx context.Context, email string, limit int) ([]Order, error)
	ListHoldings(ctx context.Context, email string) ([]Holding, error)

	// Admin queries & mutations
	ListAllUsers(ctx context.Context) ([]Account, error)
	SetUserAdmin(ctx context.Context, email string, isAdmin bool) error
	SetUserBalance(ctx context.Context, email string, balanceCents int64) error
	ListAllOrders(ctx context.Context, limit int) ([]Order, error)
}
