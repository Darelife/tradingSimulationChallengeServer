package main

import (
	"context"
	"time"
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

	SaveRegistrationChallenge(
		context.Context,
		RegistrationChallenge,
	) error

	GetRegistrationChallenge(
		context.Context,
		string,
	) (RegistrationChallenge, error)

	CompleteRegistration(
		context.Context,
		string,
		string,
		string,
	) error
}
