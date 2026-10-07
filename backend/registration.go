package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
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

func registerStart(database Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		request, err := parseRegisterRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		email := strings.ToLower(request.Email)

		if !strings.HasSuffix(email, "@goa.bits-pilani.ac.in") {
			http.Error(w, "Email must be a BITS Goa email", http.StatusBadRequest)
			return
		}

		exists, err := database.UserExists(r.Context(), email)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "User already exists", http.StatusConflict)
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

		if err := database.SaveRegistrationChallenge(r.Context(), challenge); err != nil {
			http.Error(w, "Could not save challenge", http.StatusInternalServerError)
			return
		}

		if err := sendVerificationEmail(email, code); err != nil {
			http.Error(w, "Could not send verification email", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"challenge_id": challengeID,
		})
	}
}

func registerComplete(database Database) http.HandlerFunc {
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
			http.Error(w, "Invalid registration data", http.StatusBadRequest)
			return
		}

		challenge, err := database.GetRegistrationChallenge(r.Context(), request.ChallengeID)
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

		err = database.CompleteRegistration(
			r.Context(),
			request.ChallengeID,
			string(passwordHash),
			r.RemoteAddr,
		)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Challenge expired or already used", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "Could not create user", http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(w, "Registration complete")
	}
}
