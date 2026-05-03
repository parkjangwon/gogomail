package spam

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

type Action string

const (
	ActionAccept     Action = "accept"
	ActionQuarantine Action = "quarantine"
	ActionReject     Action = "reject"
	ActionTempfail   Action = "tempfail"
)

type Verdict struct {
	Action Action
	Score  float64
	Reason string
	Tags   []string
}

type Request struct {
	Stage          smtpd.Stage
	RemoteAddr     string
	EnvelopeFrom   string
	Recipients     []string
	Parsed         message.ParsedMessage
	Authentication smtpd.AuthenticationResults
	Size           int64
}

type Relay interface {
	Check(ctx context.Context, req Request) (Verdict, error)
}

type HookOptions struct {
	Relay  Relay
	Stage  smtpd.Stage
	Shadow bool
}

func Hook(opts HookOptions) smtpd.Hook {
	stage := opts.Stage
	if stage == "" {
		stage = smtpd.StageAuthenticationChecked
	}
	return func(ctx context.Context, event smtpd.Event) error {
		if opts.Relay == nil || event.Stage != stage {
			return nil
		}
		verdict, err := opts.Relay.Check(ctx, Request{
			Stage:          event.Stage,
			EnvelopeFrom:   event.EnvelopeFrom,
			Recipients:     append([]string(nil), event.Recipients...),
			Parsed:         event.Parsed,
			Authentication: event.Authentication,
			Size:           event.Size,
		})
		if err != nil {
			if opts.Shadow {
				return nil
			}
			return fmt.Errorf("spam relay check: %w", err)
		}
		if opts.Shadow {
			return nil
		}
		switch normalizeAction(verdict.Action) {
		case ActionAccept, ActionQuarantine:
			return nil
		case ActionTempfail:
			return fmt.Errorf("temporary spam policy failure: %s", verdict.reason())
		case ActionReject:
			return fmt.Errorf("spam rejected: %s", verdict.reason())
		default:
			return fmt.Errorf("spam relay returned unsupported action %q", verdict.Action)
		}
	}
}

func normalizeAction(action Action) Action {
	if action == "" {
		return ActionAccept
	}
	return Action(strings.ToLower(strings.TrimSpace(string(action))))
}

func (v Verdict) reason() string {
	reason := strings.TrimSpace(v.Reason)
	if reason != "" {
		return reason
	}
	return "policy"
}
