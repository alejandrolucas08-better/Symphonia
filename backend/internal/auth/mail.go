package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

var ErrMailUnavailable = errors.New("email delivery is unavailable")

type Mailer interface {
	Send(context.Context, string, string, string) error
}

type SMTPMailer struct {
	Host, Port, User, Password, From string
}

func MailerFromEnv() *SMTPMailer {
	return &SMTPMailer{
		Host: os.Getenv("SMTP_HOST"), Port: os.Getenv("SMTP_PORT"),
		User: os.Getenv("SMTP_USER"), Password: os.Getenv("SMTP_PASSWORD"), From: os.Getenv("EMAIL_FROM"),
	}
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	if m.Host == "" || m.From == "" {
		return ErrMailUnavailable
	}
	from, err := mail.ParseAddress(m.From)
	if err != nil {
		return ErrMailUnavailable
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil {
		return err
	}
	port := m.Port
	if port == "" {
		port = "587"
	}
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(m.Host, port))
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if m.Host != "localhost" && m.Host != "127.0.0.1" && m.Host != "mailpit" {
		return ErrMailUnavailable
	}
	if m.User != "" {
		if err = client.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); err != nil {
			return err
		}
	}
	if err = client.Mail(from.Address); err != nil {
		return err
	}
	if err = client.Rcpt(recipient.Address); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	message := "From: " + m.From + "\r\nTo: " + recipient.Address + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")
	if _, err = w.Write([]byte(message)); err != nil {
		return err
	}
	return w.Close()
}
