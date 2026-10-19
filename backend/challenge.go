package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ChallengeTable names the SQL table backing a challenge flow. Values are
// compile-time constants in this package, never user input.
type ChallengeTable string

// challengeFlow describes one email-code challenge flow (registration,
// password reset, ...). challengeStart and challengeComplete below are the
// only handlers; each flow only supplies its differences here.
type challengeFlow struct {
	table ChallengeTable

	// start: userMustExist is the required UserExists result; if the actual
	// result differs, the handler fails with userFailureStatus/Msg.
	userMustExist     bool
	userFailureStatus int
	userFailureMsg    string

	subject         string
	instruction     string
	sendEmailErrMsg string

	// complete: complete consumes the challenge and applies the new
	// password (insert or update), returning sql.ErrNoRows if the
	// challenge is expired/already used or the user row is missing.
	invalidDataMsg string
	complete       func(ctx context.Context, challengeID string, passwordHash string, r *http.Request) error
	completeErrMsg string
	successMsg     string
}

func challengeStart(database Database, flow challengeFlow) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		email, err := parseEmailRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		exists, err := database.UserExists(r.Context(), email)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if exists != flow.userMustExist {
			http.Error(w, flow.userFailureMsg, flow.userFailureStatus)
			return
		}

		code, err := generateCode()
		if err != nil {
			http.Error(w, "Could not generate code", http.StatusInternalServerError)
			return
		}

		challengeID, err := generateID()
		if err != nil {
			http.Error(w, "Could not generate challenge", http.StatusInternalServerError)
			return
		}

		challenge := RegistrationChallenge{
			ID:        challengeID,
			Email:     email,
			CodeHash:  hashCode(code),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}

		if err := database.SaveChallenge(r.Context(), flow.table, challenge); err != nil {
			http.Error(w, "Could not save challenge", http.StatusInternalServerError)
			return
		}

		if err := sendCodeEmail(email, flow.subject, flow.instruction, code); err != nil {
			http.Error(w, flow.sendEmailErrMsg, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"challenge_id": challengeID,
		})
	}
}

func challengeComplete(database Database, flow challengeFlow) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request registerCompleteRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if request.ChallengeID == "" || len(request.Code) != 6 || len(request.Password) < 1 {
			http.Error(w, flow.invalidDataMsg, http.StatusBadRequest)
			return
		}

		challenge, err := database.GetChallenge(r.Context(), flow.table, request.ChallengeID)
		if err != nil {
			http.Error(w, "Invalid challenge", http.StatusBadRequest)
			return
		}

		if challenge.ConsumedAt != nil || time.Now().After(challenge.ExpiresAt) {
			http.Error(w, "Challenge expired or already used", http.StatusBadRequest)
			return
		}

		if subtle.ConstantTimeCompare(
			[]byte(hashCode(request.Code)),
			[]byte(challenge.CodeHash),
		) != 1 {
			http.Error(w, "Invalid code", http.StatusBadRequest)
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Could not hash password", http.StatusInternalServerError)
			return
		}

		err = flow.complete(r.Context(), request.ChallengeID, string(passwordHash), r)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Challenge expired or already used", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, flow.completeErrMsg, http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(w, flow.successMsg)
	}
}
