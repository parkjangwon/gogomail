package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/outbound"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func (r *Repository) AuthenticatePlain(ctx context.Context, _ string, username string, password string) (smtpd.SubmissionUser, error) {
	if r.db == nil {
		return smtpd.SubmissionUser{}, fmt.Errorf("database handle is required")
	}

	normalizedUsername := strings.TrimSpace(username)
	normalizedAddress := normalizedUsername
	if strings.Contains(normalizedUsername, "@") {
		var err error
		normalizedAddress, err = mail.NormalizeAddress(normalizedUsername)
		if err != nil {
			return smtpd.SubmissionUser{}, err
		}
	}

	const query = `
SELECT
  d.company_id::text,
  u.domain_id::text,
  u.id::text,
  ua.address,
  u.display_name,
  COALESCE(u.password_hash, '')
FROM users u
JOIN domains d ON d.id = u.domain_id
JOIN user_addresses ua ON ua.user_id = u.id
WHERE u.status = 'active'
  AND d.status = 'active'
  AND u.auth_source = 'local'
  AND (
    lower(u.username) = lower($1)
    OR lower(ua.address) = lower($2)
  )
ORDER BY ua.is_primary DESC
LIMIT 1`

	var user smtpd.SubmissionUser
	var passwordHash string
	if err := r.db.QueryRowContext(ctx, query, normalizedUsername, normalizedAddress).Scan(
		&user.CompanyID,
		&user.DomainID,
		&user.UserID,
		&user.Address,
		&user.DisplayName,
		&passwordHash,
	); err != nil {
		if err == sql.ErrNoRows {
			return smtpd.SubmissionUser{}, fmt.Errorf("submission user %q not found", username)
		}
		return smtpd.SubmissionUser{}, fmt.Errorf("authenticate submission user: %w", err)
	}
	if !auth.VerifyPasswordHash(password, passwordHash) {
		return smtpd.SubmissionUser{}, fmt.Errorf("invalid submission credentials")
	}
	return user, nil
}

func (r *Repository) RecordSubmitted(ctx context.Context, msg smtpd.SubmittedMessage) (string, error) {
	to := outboundFromParsed(msg.Parsed.To)
	cc := outboundFromParsed(msg.Parsed.Cc)
	bcc := submittedBccRecipients(msg.Parsed, msg.Recipients)
	recipients := len(to) + len(cc) + len(bcc)

	return r.RecordOutgoing(ctx, OutgoingMessage{
		CompanyID:    msg.User.CompanyID,
		DomainID:     msg.User.DomainID,
		UserID:       msg.User.UserID,
		RFCMessageID: emptyGeneratedMessageID(msg.Parsed.MessageID, msg.User.Address),
		Subject:      msg.Parsed.Subject,
		From: outbound.Address{
			Name:  firstNonEmpty(msg.Parsed.From.Name, msg.User.DisplayName),
			Email: msg.User.Address,
		},
		To:          to,
		Cc:          cc,
		Bcc:         bcc,
		SentAt:      msg.SubmittedAt,
		Size:        msg.Size,
		StoragePath: msg.StoragePath,
		Farm: outbound.Classify(outbound.ClassificationInput{
			RecipientCount: recipients,
		}),
	})
}

func submittedBccRecipients(parsed message.ParsedMessage, envelopeRecipients []string) []outbound.Address {
	bcc := outboundFromParsed(parsed.Bcc)
	seen := make(map[string]struct{}, len(parsed.To)+len(parsed.Cc)+len(parsed.Bcc)+len(envelopeRecipients))
	for _, addr := range parsed.To {
		seen[strings.ToLower(addr.Address)] = struct{}{}
	}
	for _, addr := range parsed.Cc {
		seen[strings.ToLower(addr.Address)] = struct{}{}
	}
	for _, addr := range parsed.Bcc {
		seen[strings.ToLower(addr.Address)] = struct{}{}
	}
	for _, recipient := range envelopeRecipients {
		normalized, err := mail.NormalizeAddress(recipient)
		if err != nil {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		bcc = append(bcc, outbound.Address{Email: normalized})
		seen[normalized] = struct{}{}
	}
	return bcc
}

func outboundFromParsed(addrs []message.Address) []outbound.Address {
	values := make([]outbound.Address, 0, len(addrs))
	for _, addr := range addrs {
		values = append(values, outbound.Address{Name: addr.Name, Email: addr.Address})
	}
	return values
}

func emptyGeneratedMessageID(messageID string, from string) string {
	if strings.TrimSpace(messageID) != "" {
		return messageID
	}
	_, domain, ok := strings.Cut(from, "@")
	if !ok {
		domain = "localhost"
	}
	return outbound.GenerateMessageID(domain)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
