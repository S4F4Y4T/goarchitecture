package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// SMTPSender sends mail via a standard SMTP server, optionally upgrading
// the connection with STARTTLS and authenticating with PLAIN auth.
// Pointing it at a real provider (SES, SendGrid, ...) in production vs. a
// local catcher like Mailpit in development is purely a config change — no
// code changes needed either way.
type SMTPSender struct {
	host        string
	port        int
	username    string
	password    string
	fromName    string
	fromAddress string
	useTLS      bool
}

func NewSMTPSender(host string, port int, username, password, fromName, fromAddress string, useTLS bool) *SMTPSender {
	return &SMTPSender{
		host:        host,
		port:        port,
		username:    username,
		password:    password,
		fromName:    fromName,
		fromAddress: fromAddress,
		useTLS:      useTLS,
	}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dialing smtp server %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("creating smtp client: %w", err)
	}
	defer client.Close()

	if s.useTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}

	if s.username != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err := client.Mail(s.fromAddress); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp RCPT: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(buildMessage(s.fromName, s.fromAddress, msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("writing smtp message body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("closing smtp message body: %w", err)
	}

	return client.Quit()
}

func buildMessage(fromName, fromAddress string, msg Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, fromAddress)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(msg.Body)
	return []byte(b.String())
}
