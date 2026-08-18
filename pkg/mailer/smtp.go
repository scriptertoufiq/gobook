package mailer

import (
	"context"
	"fmt"
	"mime"
	"net/smtp"
	"strings"
	"time"
)

// SMTPMailer delivers over SMTP using the standard library. smtp.SendMail
// upgrades to STARTTLS automatically when the server advertises it.
type SMTPMailer struct {
	addr        string
	username    string
	password    string
	fromAddress string
	fromName    string
	timeout     time.Duration
}

func NewSMTP(addr, username, password, fromAddress, fromName string) *SMTPMailer {
	return &SMTPMailer{
		addr:        addr,
		username:    username,
		password:    password,
		fromAddress: fromAddress,
		fromName:    fromName,
		timeout:     10 * time.Second,
	}
}

var _ Mailer = (*SMTPMailer)(nil)

func (m *SMTPMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	if to == "" {
		return fmt.Errorf("mailer: empty recipient")
	}

	// Empty username means no auth — that's what a local Mailpit/MailHog
	// wants, and net/smtp refuses PlainAuth over an unencrypted connection.
	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, hostOf(m.addr))
	}

	msg := m.build(to, subject, htmlBody)

	// smtp.SendMail has no context support, so run it where cancellation and
	// the deadline can still take effect on the caller's side.
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(m.addr, auth, m.fromAddress, []string{to}, msg)
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("mailer: send to %s via %s: %w", to, m.addr, err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("mailer: send to %s cancelled: %w", to, ctx.Err())
	case <-time.After(m.timeout):
		return fmt.Errorf("mailer: send to %s timed out after %s", to, m.timeout)
	}
}

// build assembles a minimal RFC 5322 message. Headers are CRLF-separated and
// the subject is Q-encoded so non-ASCII survives.
func (m *SMTPMailer) build(to, subject, htmlBody string) []byte {
	from := m.fromAddress
	if m.fromName != "" {
		from = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", m.fromName), m.fromAddress)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)

	return []byte(b.String())
}

func hostOf(addr string) string {
	if host, _, ok := strings.Cut(addr, ":"); ok {
		return host
	}
	return addr
}
