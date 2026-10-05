package main

import (
	"fmt"
	"net/http"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Server is running")
}

func main() {
	http.HandleFunc("/", home)

	fmt.Println("Listening on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
