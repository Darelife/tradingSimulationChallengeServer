package main

import (
	"fmt"
	"net"
	"net/smtp"
	"os"
)

func sendVerificationEmail(to string, code string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	if host == "" || port == "" || username == "" || password == "" || from == "" {
		return fmt.Errorf("SMTP configuration is incomplete")
	}

	auth := smtp.PlainAuth("", username, password, host)

	message := []byte(fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: Trading simulation verification code\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n"+
			"\r\n"+
			"Your verification code is: %s\r\n",
		from,
		to,
		code,
	))

	return smtp.SendMail(
		net.JoinHostPort(host, port),
		auth,
		from,
		[]string{to},
		message,
	)
}
