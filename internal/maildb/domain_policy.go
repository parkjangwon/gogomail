package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// InboundDomainPolicy implements smtpd.DomainPolicyLookup so Repository can be
// wired directly into the SMTP receiver and submission engines.
func (r *Repository) InboundDomainPolicy(ctx context.Context, domainID string) (smtpd.InboundDomainPolicy, error) {
	view, err := r.DomainPolicy(ctx, domainID)
	if err != nil {
		return smtpd.InboundDomainPolicy{}, err
	}
	return smtpd.InboundDomainPolicy{
		InboundMode:             view.InboundMode,
		MaxRecipientsPerMessage: view.MaxRecipientsPerMessage,
		MaxMessageBytes:         view.MaxMessageBytes,
	}, nil
}

func (r *Repository) DomainPolicy(ctx context.Context, domainID string) (DomainPolicyView, error) {
	if r.db == nil {
		return DomainPolicyView{}, fmt.Errorf("database handle is required")
	}
	domainID = strings.TrimSpace(domainID)
	if domainID == "" {
		return DomainPolicyView{}, fmt.Errorf("domain id is required")
	}

	const query = `
SELECT COALESCE(settings->'policy', '{}'::jsonb), updated_at
FROM domains
WHERE id = $1
LIMIT 1`

	var raw []byte
	var updatedAt time.Time
	if err := r.db.QueryRowContext(ctx, query, domainID).Scan(&raw, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return DomainPolicyView{}, fmt.Errorf("domain %q not found", domainID)
		}
		return DomainPolicyView{}, fmt.Errorf("read domain policy: %w", err)
	}
	return domainPolicyFromJSON(domainID, raw, updatedAt)
}

func domainPolicyFromJSON(domainID string, raw []byte, updatedAt time.Time) (DomainPolicyView, error) {
	policy := DomainPolicyView{
		DomainID:     strings.TrimSpace(domainID),
		InboundMode:  "inherit",
		OutboundMode: "inherit",
		UpdatedAt:    updatedAt,
	}
	if len(raw) > 0 && string(raw) != "{}" {
		if err := json.Unmarshal(raw, &policy); err != nil {
			return DomainPolicyView{}, fmt.Errorf("decode domain policy: %w", err)
		}
	}
	policy.DomainID = strings.TrimSpace(domainID)
	inboundMode, err := normalizeDomainPolicyMode(policy.InboundMode)
	if err != nil {
		return DomainPolicyView{}, fmt.Errorf("stored inbound_mode %w", err)
	}
	outboundMode, err := normalizeDomainPolicyMode(policy.OutboundMode)
	if err != nil {
		return DomainPolicyView{}, fmt.Errorf("stored outbound_mode %w", err)
	}
	policy.InboundMode = inboundMode
	policy.OutboundMode = outboundMode
	policy.UpdatedAt = updatedAt
	if policy.MaxRecipientsPerMessage < 0 {
		return DomainPolicyView{}, fmt.Errorf("stored max_recipients_per_message must not be negative")
	}
	if policy.MaxMessageBytes < 0 {
		return DomainPolicyView{}, fmt.Errorf("stored max_message_bytes must not be negative")
	}
	if policy.MaxAttachmentBytes < 0 {
		return DomainPolicyView{}, fmt.Errorf("stored max_attachment_bytes must not be negative")
	}
	return policy, nil
}

func (r *Repository) DomainPolicyForUser(ctx context.Context, userID string) (DomainPolicyView, error) {
	if r.db == nil {
		return DomainPolicyView{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DomainPolicyView{}, fmt.Errorf("user id is required")
	}

	const query = `
SELECT d.id::text, COALESCE(d.settings->'policy', '{}'::jsonb), d.updated_at
FROM users u
JOIN domains d ON d.id = u.domain_id
WHERE u.id = $1
LIMIT 1`

	var domainID string
	var raw []byte
	var updatedAt time.Time
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&domainID, &raw, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return DomainPolicyView{}, fmt.Errorf("user %q not found for domain policy", userID)
		}
		return DomainPolicyView{}, fmt.Errorf("read user domain policy: %w", err)
	}
	return domainPolicyFromJSON(domainID, raw, updatedAt)
}
