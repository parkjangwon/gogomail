package milterhook

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/milter"
	"github.com/gogomail/gogomail/internal/spam"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// SpamConfig holds spam filtering configuration.
type SpamConfig struct {
	Enabled    bool
	Filter     spam.Filter
	Engine     *spam.DecisionEngine
	ShadowMode bool
}

// SpamHook integrates spam filtering with Milter protocol.
// Spam verdicts are converted to Milter actions. X-Spam headers are added
// by the mail storage layer after filtering.
func SpamHook(cfg SpamConfig) smtpd.Hook {
	return func(ctx context.Context, event smtpd.Event) error {
		if !cfg.Enabled || cfg.Filter == nil || cfg.Engine == nil {
			return nil
		}

		// Run spam check at authentication-checked stage (after initial filtering)
		if event.Stage != smtpd.StageAuthenticationChecked {
			return nil
		}

		// Extract subject for spam analysis (minimal metadata)
		subject := event.Parsed.Subject
		from := event.Parsed.From.Address

		// Build a text representation from parsed message for analysis
		messageText := buildMessageText(event.Parsed)

		// Call spam filter
		score, err := cfg.Filter.Check(ctx, from, event.Recipients, []byte(messageText))
		if err != nil {
			if cfg.ShadowMode {
				// Log error but don't fail the message in shadow mode
				return nil
			}
			return fmt.Errorf("spam filter check: %w", err)
		}

		// Decide action based on score
		verdict := cfg.Engine.Decide(score)

		// Store verdict in event for downstream processing
		// (In a real implementation, this would be stored in event context)
		_ = verdict
		_ = subject

		// Apply verdict (may reject/tempfail the message)
		if cfg.ShadowMode {
			// In shadow mode, log verdict but allow message through
			return nil
		}

		switch verdict.Action {
		case spam.ActionAccept, spam.ActionQuarantine:
			// Message accepted (quarantine handled by router, not here)
			return nil

		case spam.ActionTempfail:
			return fmt.Errorf("temporary spam policy failure: %s", verdict.Reason)

		case spam.ActionReject:
			return fmt.Errorf("message rejected by spam filter: %s", verdict.Reason)

		default:
			return fmt.Errorf("spam filter returned unknown action: %v", verdict.Action)
		}
	}
}

// buildMessageText constructs a text representation of parsed message for analysis.
func buildMessageText(parsed message.ParsedMessage) string {
	var parts []string
	if parsed.Subject != "" {
		parts = append(parts, "Subject: "+parsed.Subject)
	}
	if parsed.TextBody != "" {
		parts = append(parts, parsed.TextBody)
	}
	return strings.Join(parts, "\n")
}

// MilterSpamVerdict adapts spam verdict to Milter action.
// Returns the appropriate Milter action (Accept, Reject, TempFail).
func MilterSpamVerdict(verdict spam.Verdict) milter.Action {
	switch verdict.Action {
	case spam.ActionAccept:
		return milter.ActionAccept

	case spam.ActionQuarantine:
		// Quarantine is typically handled by headers/routing, not Milter rejection
		// Return Accept and let mail service handle quarantine
		return milter.ActionAccept

	case spam.ActionTempfail:
		return milter.ActionTempfail

	case spam.ActionReject:
		return milter.ActionReject

	default:
		return milter.ActionAccept
	}
}

// SpamVerdictHeaders returns X-Spam headers for the verdict.
// These should be added to the message during storage.
func SpamVerdictHeaders(verdict spam.Verdict, score spam.SpamScore) []map[string]string {
	headers := make([]map[string]string, 0)

	// X-Spam-Score header
	scoreStr := strconv.FormatFloat(verdict.Score, 'f', 2, 64)
	headers = append(headers, map[string]string{
		"X-Spam-Score": scoreStr,
	})

	// X-Spam-Status header
	statusValue := "NO"
	if verdict.Action == spam.ActionReject || verdict.Action == spam.ActionQuarantine {
		statusValue = "YES"
	}
	headers = append(headers, map[string]string{
		"X-Spam-Status": statusValue,
	})

	// X-Spam-Report header with rules
	if len(score.Rules) > 0 {
		report := fmt.Sprintf("score=%s rules=%s", scoreStr, strings.Join(score.Rules, ","))
		headers = append(headers, map[string]string{
			"X-Spam-Report": report,
		})
	}

	return headers
}
