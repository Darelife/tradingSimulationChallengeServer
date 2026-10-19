package main

import (
	"errors"
	"time"
)

// Money is handled as integer paise (1 rupee = 100 paise) everywhere, so
// balances never suffer float rounding errors. The *Cents suffix is a
// historical name for paise units.
const StartingBalanceCents = 10_000_000 // ₹1,00,000

// SessionCookieName is the cookie holding the session token.
const SessionCookieName = "session"

// Trading errors. Handlers map them to HTTP statuses.
var (
	ErrUnknownSymbol      = errors.New("unknown symbol")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrInsufficientShares = errors.New("insufficient shares")
	ErrInvalidQuantity    = errors.New("quantity must be a positive integer")
)

// Company is one tradable security.
type Company struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	// complementary_of names the symbol whose price moves inversely to this
	// one on targeted shocks; empty for independent companies.
	ComplementaryOf string `json:"complementary_of,omitempty"`
	BasePriceCents  int64  `json:"-"`
}

// PricePoint is one historical price sample.
type PricePoint struct {
	Symbol     string `json:"symbol"`
	PriceCents int64  `json:"price_cents"`
	Timestamp  int64  `json:"ts"` // unix seconds
}

// Account is a user's trading account state.
type Account struct {
	Email        string `json:"-"`
	BalanceCents int64  `json:"balance_cents"`
	IsAdmin      bool   `json:"-"`
}

// Order is an executed trade.
type Order struct {
	ID         int64  `json:"id"`
	Email      string `json:"-"`
	Symbol     string `json:"symbol"`
	Side       string `json:"side"` // "buy" | "sell"
	Quantity   int64  `json:"quantity"`
	PriceCents int64  `json:"price_cents"` // unit price at execution
	TotalCents int64  `json:"total_cents"`
	CreatedAt  int64  `json:"created_at"` // unix seconds
}

// Holding is a user's position in one symbol.
type Holding struct {
	Symbol       string `json:"symbol"`
	Quantity     int64  `json:"quantity"`
	AvgCostCents int64  `json:"avg_cost_cents"`
}

// Session is an authenticated browser session.
type Session struct {
	Token     string
	Email     string
	ExpiresAt time.Time
}
