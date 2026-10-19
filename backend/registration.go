package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

type registerRequest struct {
	Email string `json:"email"`
}

type registerCompleteRequest struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
	Password    string `json:"password"`
}

func parseRegisterRequest(r *http.Request) (registerRequest, error) {
	var request registerRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		return request, fmt.Errorf("invalid JSON")
	}

	request.Email = strings.TrimSpace(request.Email)

	if request.Email == "" {
		return request, fmt.Errorf("email is required")
	}

	return request, nil
}

func parseEmailRequest(r *http.Request) (string, error) {
	request, err := parseRegisterRequest(r)
	if err != nil {
		return "", err
	}

	email := strings.ToLower(request.Email)

	if !strings.HasSuffix(email, "@goa.bits-pilani.ac.in") {
		return "", fmt.Errorf("Email must be a BITS Goa email")
	}

	return email, nil
}

func generateCode() (string, error) {
	max := big.NewInt(1000000)

	number, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", number.Int64()), nil
}

func hashCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", hash[:])
}

func generateID() (string, error) {
	bytes := make([]byte, 32)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", bytes), nil
}

func registerFlow(database Database) challengeFlow {
	return challengeFlow{
		table:             registrationChallenges,
		userMustExist:     false,
		userFailureStatus: http.StatusConflict,
		userFailureMsg:    "User already exists",
		subject:           "Trading simulation verification code",
		instruction:       "Your verification code is:",
		sendEmailErrMsg:   "Could not send verification email",
		invalidDataMsg:    "Invalid registration data",
		complete: func(ctx context.Context, challengeID string, passwordHash string, r *http.Request) error {
			return database.CompleteRegistration(
				ctx,
				challengeID,
				passwordHash,
				r.RemoteAddr,
				false,
			)
		},
		completeErrMsg: "Could not create user",
		successMsg:     "Registration complete",
	}
}

func registerStart(database Database) http.HandlerFunc {
	return challengeStart(database, registerFlow(database))
}

func registerComplete(database Database) http.HandlerFunc {
	return challengeComplete(database, registerFlow(database))
}
