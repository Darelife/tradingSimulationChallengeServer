package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Server is running")
}

type registerRequest struct {
	Email string `json:"email"`
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

func registerStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	request, err := parseRegisterRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintln(w, "Registration started for", request.Email)

	code, err := generateCode()
	if err != nil {
		http.Error(w, "Could not generate code", http.StatusInternalServerError)
		return
	}

	fmt.Println("Verification code:", code)
}

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/register/start", registerStart)

	fmt.Println("Listening on http://localhost:8080")

	database, err := NewSQLiteDatabase("trading.db")
	if err != nil {
		panic(err)
	}
	defer database.Close()

	http.ListenAndServe(":8080", nil)
}
