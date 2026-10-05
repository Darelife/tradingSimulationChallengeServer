package main

import (
	"context"
	"time"
)

type RegistrationChallenge struct {
	ID        string
	Email     string
	CodeHash  string
	ExpiresAt time.Time
}

type Database interface {
	SaveRegistrationChallenge(context.Context, RegistrationChallenge) error
	GetRegistrationChallenge(context.Context, string) (RegistrationChallenge, error)
	DeleteRegistrationChallenge(context.Context, string) error
}
