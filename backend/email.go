package main

import (
	"fmt"
	"net"
	"net/smtp"
	"os"
)

func sendCodeEmail(to string, subject string, instruction string, code string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")

	if host == "" || port == "" || username == "" || password == "" || from == "" {
		return fmt.Errorf("SMTP configuration is incomplete")
	}

	auth := smtp.PlainAuth("", username, password, host)

	message := fmt.Appendf(nil,
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n"+
			"\r\n"+
			"%s %s\r\n",
		from,
		to,
		subject,
		instruction,
		code,
	)

	return smtp.SendMail(
		net.JoinHostPort(host, port),
		auth,
		from,
		[]string{to},
		message,
	)
}
