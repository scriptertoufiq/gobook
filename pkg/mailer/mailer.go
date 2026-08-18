// Package mailer sends transactional email.
package mailer

import "context"

// Mailer is an interface for the same reason repositories are: it lets the
// service layer be tested without a live SMTP server. There is exactly one
// production implementation, SMTPMailer.
type Mailer interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}
