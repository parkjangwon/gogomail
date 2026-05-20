package mailservice

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// SystemEmailSender sends transactional system emails (password reset, welcome,
// invite, quota alerts). All methods are best-effort: the caller is responsible
// for retries. If GOGOMAIL_SYSTEM_EMAIL_FROM is empty, sending is skipped and
// a log line is emitted instead.
type SystemEmailSender interface {
	SendPasswordReset(ctx context.Context, toEmail, resetURL string) error
	SendWelcome(ctx context.Context, toEmail, displayName string) error
	SendInvite(ctx context.Context, toEmail, inviteURL string) error
	SendQuotaAlert(ctx context.Context, toEmail string, pct int) error
}

// SMTPSystemEmailSender implements SystemEmailSender via net/smtp.
type SMTPSystemEmailSender struct {
	fromAddr string // empty → log-only mode
	smtpAddr string // host:port
	smtpAuth smtp.Auth
}

// SystemEmailSenderConfig configures an SMTPSystemEmailSender from environment
// variables so that callers have a single constructor to wire up.
//
// Required env:
//
//	GOGOMAIL_SYSTEM_EMAIL_FROM  – envelope/header From address
//	GOGOMAIL_SYSTEM_SMTP_ADDR  – SMTP relay, e.g. "127.0.0.1:25"
//
// Optional env (for SMTP AUTH LOGIN / PLAIN):
//
//	GOGOMAIL_SYSTEM_SMTP_USER
//	GOGOMAIL_SYSTEM_SMTP_PASS
//
// If GOGOMAIL_SYSTEM_EMAIL_FROM is empty the sender operates in log-only mode:
// emails are formatted and logged at Debug level but not transmitted.
func NewSMTPSystemEmailSenderFromEnv() *SMTPSystemEmailSender {
	from := strings.TrimSpace(os.Getenv("GOGOMAIL_SYSTEM_EMAIL_FROM"))
	addr := strings.TrimSpace(os.Getenv("GOGOMAIL_SYSTEM_SMTP_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:25"
	}

	var auth smtp.Auth
	user := os.Getenv("GOGOMAIL_SYSTEM_SMTP_USER")
	pass := os.Getenv("GOGOMAIL_SYSTEM_SMTP_PASS")
	if user != "" {
		host := addr
		if idx := strings.LastIndex(addr, ":"); idx >= 0 {
			host = addr[:idx]
		}
		auth = smtp.PlainAuth("", user, pass, host)
	}

	return &SMTPSystemEmailSender{
		fromAddr: from,
		smtpAddr: addr,
		smtpAuth: auth,
	}
}

// SendPasswordReset sends a password reset link to toEmail.
func (s *SMTPSystemEmailSender) SendPasswordReset(_ context.Context, toEmail, resetURL string) error {
	subject := "Reset your password"
	body := fmt.Sprintf(
		"You requested a password reset.\r\n\r\n"+
			"Click the link below to choose a new password (valid for 1 hour):\r\n\r\n"+
			"%s\r\n\r\n"+
			"If you did not request this, you can safely ignore this email.\r\n",
		resetURL,
	)
	return s.send(toEmail, subject, body)
}

// SendWelcome sends a welcome email to a newly created user.
func (s *SMTPSystemEmailSender) SendWelcome(_ context.Context, toEmail, displayName string) error {
	subject := "Welcome to Gogomail"
	body := fmt.Sprintf(
		"Hi %s,\r\n\r\nYour Gogomail account is ready. You can log in now.\r\n",
		displayName,
	)
	return s.send(toEmail, subject, body)
}

// SendInvite sends an invitation link.
func (s *SMTPSystemEmailSender) SendInvite(_ context.Context, toEmail, inviteURL string) error {
	subject := "You've been invited to Gogomail"
	body := fmt.Sprintf(
		"You have been invited to join Gogomail.\r\n\r\n"+
			"Accept your invitation here:\r\n\r\n%s\r\n",
		inviteURL,
	)
	return s.send(toEmail, subject, body)
}

// SendQuotaAlert notifies a user that their mailbox is pct% full.
func (s *SMTPSystemEmailSender) SendQuotaAlert(_ context.Context, toEmail string, pct int) error {
	subject := fmt.Sprintf("Mailbox storage alert: %d%% used", pct)
	body := fmt.Sprintf(
		"Your mailbox is now %d%% full.\r\n\r\n"+
			"Please delete old messages or contact your administrator to increase your quota.\r\n",
		pct,
	)
	return s.send(toEmail, subject, body)
}

// send formats and transmits a plain-text email. In log-only mode (fromAddr
// empty) it logs at Debug level and returns nil.
func (s *SMTPSystemEmailSender) send(toEmail, subject, body string) error {
	if s.fromAddr == "" {
		slog.Debug("system email skipped (GOGOMAIL_SYSTEM_EMAIL_FROM not set)",
			"to", toEmail,
			"subject", subject,
		)
		return nil
	}

	msg := buildRFC2822Message(s.fromAddr, toEmail, subject, body)
	if err := smtp.SendMail(s.smtpAddr, s.smtpAuth, s.fromAddr, []string{toEmail}, []byte(msg)); err != nil {
		return fmt.Errorf("system email send to %s: %w", toEmail, err)
	}
	slog.Info("system email sent", "to", toEmail, "subject", subject)
	return nil
}

// buildRFC2822Message builds a minimal RFC 2822 plain-text message.
func buildRFC2822Message(from, to, subject, body string) string {
	now := time.Now().UTC().Format(time.RFC1123Z)
	var sb strings.Builder
	sb.WriteString("Date: " + now + "\r\n")
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}
