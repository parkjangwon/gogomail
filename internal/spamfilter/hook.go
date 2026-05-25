package spamfilter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/dnsbl"
	"github.com/gogomail/gogomail/internal/maildb"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type Resolver interface {
	Resolve(ctx context.Context, userID, domainID, companyID string, key string) (json.RawMessage, error)
}

type Options struct {
	Resolver    Resolver
	Logger      *maildb.MailFlowLogWriter
	Engine      Engine
	RBLResolver dnsbl.Resolver
}

func Hook(opts Options) smtpd.Hook {
	engine := opts.Engine
	if engine == (Engine{}) {
		engine = NewEngine()
	}
	return func(ctx context.Context, event smtpd.Event) error {
		if event.Stage != smtpd.StageDedupChecked || event.Duplicate {
			return nil
		}
		policy := DefaultPolicy()
		if opts.Resolver != nil {
			raw, err := opts.Resolver.Resolve(ctx, event.Mailbox.UserID, event.Mailbox.DomainID, event.Mailbox.CompanyID, PolicyConfigKey)
			if err == nil {
				if decoded, decodeErr := DecodePolicy(raw); decodeErr == nil {
					policy = decoded
				}
			} else if !errors.Is(err, configstore.ErrConfigNotFound) {
				return fmt.Errorf("resolve spam filter policy: %w", err)
			}
		}
		decision := engine.Evaluate(policy, event)
		if rblDecision, ok := checkRBL(ctx, opts.RBLResolver, policy, event, decision); ok {
			decision = rblDecision
		}
		if decision.Action == ActionAccept {
			if decision.Score > 0 {
				_ = logDecision(ctx, opts.Logger, event, decision, maildb.MailFlowStatusReceived)
			}
			return nil
		}
		if decision.Action == ActionQuarantine {
			return nil
		}
		status := maildb.MailFlowStatusFiltered
		if decision.Action == ActionReject {
			status = maildb.MailFlowStatusRejected
		}
		_ = logDecision(ctx, opts.Logger, event, decision, status)
		switch decision.Action {
		case ActionTempfail:
			return fmt.Errorf("temporary spam filter failure: %s", cleanReason(decision.Reason))
		case ActionReject:
			return fmt.Errorf("spam filter rejected message: %s", cleanReason(decision.Reason))
		default:
			return nil
		}
	}
}

func checkRBL(ctx context.Context, resolver dnsbl.Resolver, policy Policy, event smtpd.Event, current Decision) (Decision, bool) {
	policy = NormalizePolicy(policy)
	if !policy.RBLCheckEnabled || len(policy.RBLZones) == 0 || current.Action == ActionReject || (current.Action == ActionQuarantine && !policy.RBLRejectEnabled) || isAllowlisted(policy, event) {
		return Decision{}, false
	}
	ip := remoteIP(event.RemoteAddr)
	if ip == "" {
		return Decision{}, false
	}
	if resolver == nil {
		resolver = dnsbl.NewResolverWithTimeout(2 * time.Second)
	}
	result, err := dnsbl.NewChecker(policy.RBLZones, resolver).CheckAddr(ip)
	if err != nil || !result.Listed {
		return Decision{}, false
	}
	score := current.Score + 5
	rules := append([]string{}, current.Rules...)
	rules = append(rules, "RBL_LISTED:"+result.Zone)
	if policy.RBLRejectEnabled {
		return Decision{
			Action: ActionReject,
			Score:  score,
			Reason: "remote IP listed in RBL",
			Rules:  rules,
		}, true
	}
	return decisionForPolicy(policy, score, "remote IP listed in RBL", rules), true
}

func isAllowlisted(policy Policy, event smtpd.Event) bool {
	from := firstNonEmpty(event.Parsed.From.Address, event.EnvelopeFrom)
	return matchesAddressList(from, policy.AllowedSenders)
}

type Recorder struct {
	Next     smtpd.MessageRecorder
	Resolver Resolver
	Logger   *maildb.MailFlowLogWriter
	Engine   Engine
}

func (r Recorder) Record(ctx context.Context, msg smtpd.ReceivedMessage) (string, error) {
	if r.Next == nil {
		return "", fmt.Errorf("spamfilter recorder next recorder is required")
	}
	policy := DefaultPolicy()
	if r.Resolver != nil {
		raw, err := r.Resolver.Resolve(ctx, msg.Mailbox.UserID, msg.Mailbox.DomainID, msg.Mailbox.CompanyID, PolicyConfigKey)
		if err == nil {
			if decoded, decodeErr := DecodePolicy(raw); decodeErr == nil {
				policy = decoded
			}
		} else if !errors.Is(err, configstore.ErrConfigNotFound) {
			return "", fmt.Errorf("resolve spam filter policy: %w", err)
		}
	}
	event := smtpd.Event{
		Stage:          smtpd.StageDedupChecked,
		RemoteAddr:     "",
		EnvelopeFrom:   msg.EnvelopeFrom,
		Mailbox:        msg.Mailbox,
		Recipients:     []string{msg.Mailbox.Address},
		StoragePath:    msg.StoragePath,
		Parsed:         msg.Parsed,
		Authentication: msg.Authentication,
		ReceivedAt:     msg.ReceivedAt,
		Size:           msg.Size,
	}
	engine := r.Engine
	if engine == (Engine{}) {
		engine = NewEngine()
	}
	decision := engine.Evaluate(policy, event)
	if decision.Action == ActionQuarantine {
		msg.FolderSystemType = "spam"
		msg.SpamScore = &decision.Score
		_ = logDecision(ctx, r.Logger, event, decision, maildb.MailFlowStatusFiltered)
	}
	return r.Next.Record(ctx, msg)
}

func logDecision(ctx context.Context, writer *maildb.MailFlowLogWriter, event smtpd.Event, decision Decision, status maildb.MailFlowStatus) error {
	if writer == nil {
		return nil
	}
	now := time.Now().UTC()
	score := decision.Score
	return writer.InsertInbound(ctx, maildb.MailFlowLogEntry{
		CompanyID:      event.Mailbox.CompanyID,
		DomainID:       event.Mailbox.DomainID,
		UserID:         event.Mailbox.UserID,
		RFCMessageID:   event.Parsed.MessageID,
		FromAddr:       event.Parsed.From.Address,
		FromName:       event.Parsed.From.Name,
		ToAddrs:        event.Recipients,
		Subject:        event.Parsed.Subject,
		FlowStatus:     string(status),
		EnhancedStatus: string(decision.Action),
		ErrorMessage:   cleanReason(decision.Reason + " " + strings.Join(decision.Rules, ",")),
		SpamScore:      &score,
		DKIMResult:     string(event.Authentication.DKIM.Result),
		SPFResult:      string(event.Authentication.SPF.Result),
		DMARCResult:    string(event.Authentication.DMARC.Result),
		Transport:      "smtp",
		Size:           event.Size,
		ReceivedAt:     timePtr(event.ReceivedAt),
		ProcessedAt:    &now,
		InReplyTo:      event.Parsed.InReplyTo,
		References:     strings.Join(event.Parsed.References, " "),
		IPAddress:      remoteIP(event.RemoteAddr),
		MailFrom:       event.EnvelopeFrom,
		RcptTo:         strings.Join(event.Recipients, ","),
	})
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func cleanReason(reason string) string {
	reason = strings.TrimSpace(reason)
	reason = strings.NewReplacer("\r", " ", "\n", " ").Replace(reason)
	if len(reason) > 500 {
		return reason[:500]
	}
	if reason == "" {
		return "policy"
	}
	return reason
}
