package main

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"time"

	_ "net/http/pprof"

	"github.com/joho/godotenv"
)

var (
	from     string
	password string
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading environment: %v", err)
	}
	from = os.Getenv("MAIL")
	password = os.Getenv("PASSWD")
}

func sendEmail(toList []string) error {
	const (
		host = "smtp.gmail.com"
		port = "587"
	)

	subject := "Subject: Test Email\r\n"
	body := "Hello, this is a test email sent using Go! lol\r\n"

	message := []byte(subject + "\r\n" + body)

	auth := smtp.PlainAuth("", from, password, host)
	return smtp.SendMail(host+":"+port, auth, from, toList, message)
}

func main() {
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	toList := []string{"lidecih255@rustetic.com"}

	start := time.Now()
	if err := sendEmail(toList); err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}
	fmt.Printf("Email sending time: %s\n", time.Since(start))
}
