package auth

import (
	"context"
	"os"
	"testing"
)

func TestSMTPMailerIntegration(t *testing.T) {
	host := os.Getenv("TEST_SMTP_HOST")
	if host == "" {
		t.Skip("TEST_SMTP_HOST is not set")
	}
	mailer := &SMTPMailer{Host: host, Port: os.Getenv("TEST_SMTP_PORT"), From: "Symphonia <no-reply@symphonia.local>"}
	if err := mailer.Send(context.Background(), "recipient@example.com", "Symphonia SMTP test", "Test message from the authentication integration suite."); err != nil {
		t.Fatal(err)
	}
}
