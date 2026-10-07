package main

import (
	"fmt"
	"log"
	"net/http"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Server is running")
}

func main() {
	database, err := NewSQLiteDatabase("trading.db")
	if err != nil {
		panic(err)
	}
	defer database.Close()

	http.HandleFunc("/", home)
	http.HandleFunc("/register/start", registerStart(database))
	http.HandleFunc("/register/complete", registerComplete(database))

	fmt.Println("Listening on http://localhost:8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
