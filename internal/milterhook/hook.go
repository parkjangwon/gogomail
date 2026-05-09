package milterhook

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gogomail/gogomail/internal/milter"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

// Dialer creates a new milter.Client for a single message filter session.
type Dialer func(ctx context.Context) (*milter.Client, error)

// NetworkDialer returns a Dialer that dials the given TCP address.
func NetworkDialer(address string, timeout time.Duration) Dialer {
	return func(ctx context.Context) (*milter.Client, error) {
		return milter.Dial(ctx, "tcp", address, timeout)
	}
}

// HookOptions configures the milter hook.
type HookOptions struct {
	Dialer Dialer
}

// Hook returns a smtpd.Hook that runs the milter filter at StageParsed.
// Returns nil (disabled) when opts.Dialer is nil.
func Hook(opts HookOptions) smtpd.Hook {
	return func(ctx context.Context, event smtpd.Event) error {
		if event.Stage != smtpd.StageParsed || opts.Dialer == nil {
			return nil
		}
		return runMilter(ctx, event, opts.Dialer)
	}
}

func runMilter(ctx context.Context, event smtpd.Event, dial Dialer) error {
	c, err := dial(ctx)
	if err != nil {
		return fmt.Errorf("milter: dial: %w", err)
	}
	defer c.Close()

	if err := c.Negotiate(ctx); err != nil {
		return fmt.Errorf("milter: negotiate: %w", err)
	}

	host, portStr, err := net.SplitHostPort(event.RemoteAddr)
	if err != nil {
		host = event.RemoteAddr
		portStr = "0"
	}
	port := uint16(0)
	if p, err := strconv.Atoi(portStr); err == nil {
		port = uint16(p)
	}
	family := milter.FamilyUnknown
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			family = milter.FamilyIPv4
		} else {
			family = milter.FamilyIPv6
		}
	}

	if action, err := c.Connect(ctx, host, family, port, host); err != nil {
		return fmt.Errorf("milter: connect: %w", err)
	} else if err := verdictError(action); err != nil {
		return err
	}

	if action, err := c.MailFrom(ctx, event.EnvelopeFrom); err != nil {
		return fmt.Errorf("milter: mail from: %w", err)
	} else if err := verdictError(action); err != nil {
		return err
	}

	for _, rcpt := range event.Recipients {
		if action, err := c.RcptTo(ctx, rcpt); err != nil {
			return fmt.Errorf("milter: rcpt to: %w", err)
		} else if err := verdictError(action); err != nil {
			return err
		}
	}

	for _, h := range synthesizeHeaders(event) {
		if action, err := c.Header(ctx, h[0], h[1]); err != nil {
			return fmt.Errorf("milter: header: %w", err)
		} else if err := verdictError(action); err != nil {
			return err
		}
	}

	if action, err := c.EndOfHeaders(ctx); err != nil {
		return fmt.Errorf("milter: eoh: %w", err)
	} else if err := verdictError(action); err != nil {
		return err
	}

	if event.Parsed.TextBody != "" {
		if action, err := c.BodyChunk(ctx, []byte(event.Parsed.TextBody)); err != nil {
			return fmt.Errorf("milter: body: %w", err)
		} else if err := verdictError(action); err != nil {
			return err
		}
	}

	action, err := c.EndOfMessage(ctx)
	if err != nil {
		return fmt.Errorf("milter: eom: %w", err)
	}
	if err := verdictError(action); err != nil {
		return err
	}

	_ = c.Quit(ctx)
	return nil
}

func verdictError(action milter.Action) error {
	switch action {
	case milter.ActionReject:
		return fmt.Errorf("milter: message rejected")
	case milter.ActionTempfail:
		return fmt.Errorf("milter: message temporarily rejected")
	default:
		return nil
	}
}

func synthesizeHeaders(event smtpd.Event) [][2]string {
	p := event.Parsed
	var headers [][2]string
	if p.MessageID != "" {
		headers = append(headers, [2]string{"Message-ID", "<" + p.MessageID + ">"})
	}
	if p.Subject != "" {
		headers = append(headers, [2]string{"Subject", p.Subject})
	}
	if p.From.Address != "" {
		headers = append(headers, [2]string{"From", p.From.Address})
	}
	if !p.Date.IsZero() {
		headers = append(headers, [2]string{"Date", p.Date.Format("Mon, 02 Jan 2006 15:04:05 -0700")})
	}
	return headers
}
