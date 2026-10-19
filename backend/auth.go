package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userEmailContextKey contextKey = "userEmail"

// GetUserCredentials fetches password hash and admin status for an email.
func (database *SQLiteDatabase) GetUserCredentials(
	ctx context.Context,
	email string,
) (string, bool, error) {
	var passwordHash string
	var isAdminInt int

	query := `
		SELECT u.password_hash, COALESCE(a.is_admin, 0)
		FROM users u
		LEFT JOIN accounts a ON u.email = a.email
		WHERE u.email = ?
	`
	err := database.db.QueryRowContext(ctx, query, strings.ToLower(email)).Scan(&passwordHash, &isAdminInt)
	if err != nil {
		return "", false, err
	}

	return passwordHash, isAdminInt == 1, nil
}

type credentialsPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// extractCredentials retrieves email and password from:
// 1. JSON body fields ("email" and "password")
// 2. HTTP Basic Auth header ("Authorization: Basic ...")
// 3. Query parameters ("?email=...&password=...")
func extractCredentials(r *http.Request) (string, string, error) {
	// 1. Check HTTP Basic Auth
	if user, pass, ok := r.BasicAuth(); ok && user != "" && pass != "" {
		return strings.ToLower(strings.TrimSpace(user)), pass, nil
	}

	// 2. Check query params
	qEmail := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	qPass := r.URL.Query().Get("password")
	if qEmail != "" && qPass != "" {
		return qEmail, qPass, nil
	}

	// 3. Check JSON request body if present
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			// Restore request body for downstream handlers
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			var creds credentialsPayload
			if err := json.Unmarshal(bodyBytes, &creds); err == nil {
				e := strings.ToLower(strings.TrimSpace(creds.Email))
				if e != "" && creds.Password != "" {
					return e, creds.Password, nil
				}
			}
		}
	}

	return "", "", errors.New("missing email or password credentials")
}

// authenticateRequest validates email and password against the database for every single request.
func authenticateRequest(database Database, r *http.Request) (string, bool, error) {
	email, password, err := extractCredentials(r)
	if err != nil {
		return "", false, err
	}

	hash, isAdmin, err := database.GetUserCredentials(r.Context(), email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, errors.New("invalid email or password")
	}
	if err != nil {
		return "", false, errors.New("database authentication error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", false, errors.New("invalid email or password")
	}

	return email, isAdmin, nil
}

// requireAuth enforces email & password verification on every incoming request.
func requireAuth(database Database, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, _, err := authenticateRequest(database, r)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userEmailContextKey, email)
		next(w, r.WithContext(ctx))
	}
}

// requireAdmin enforces email & password verification and ensures the user is an admin.
func requireAdmin(database Database, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, isAdmin, err := authenticateRequest(database, r)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if !isAdmin {
			// Double check accounts table
			acct, err := database.GetAccount(r.Context(), email)
			if err != nil || !acct.IsAdmin {
				http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), userEmailContextKey, email)
		next(w, r.WithContext(ctx))
	}
}
