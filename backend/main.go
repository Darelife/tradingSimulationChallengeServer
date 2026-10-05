package main

import (
	"fmt"
	"net/http"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Server is running")
}

func registerStart(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Registration started")
}

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/register/start", registerStart)

	fmt.Println("Listening on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
