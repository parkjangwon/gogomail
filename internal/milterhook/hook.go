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

// Client is the interface that milter clients must implement.
type Client interface {
	Negotiate(ctx context.Context) error
	Connect(ctx context.Context, hostname string, family byte, port uint16, addr string) (milter.Action, error)
	MailFrom(ctx context.Context, from string) (milter.Action, error)
	RcptTo(ctx context.Context, to string) (milter.Action, error)
	Header(ctx context.Context, name, value string) (milter.Action, error)
	EndOfHeaders(ctx context.Context) (milter.Action, error)
	BodyChunk(ctx context.Context, chunk []byte) (milter.Action, error)
	EndOfMessage(ctx context.Context) (milter.Action, error)
	Quit(ctx context.Context) error
	Close() error
}

// Dialer creates a new Client for a single message filter session.
type Dialer func(ctx context.Context) (Client, error)

// NetworkDialer returns a Dialer that dials the given TCP address.
func NetworkDialer(address string, timeout time.Duration) Dialer {
	return func(ctx context.Context) (Client, error) {
		return milter.Dial(ctx, "tcp", address, timeout)
	}
}

// PoolDialer returns a Dialer that uses a connection pool with circuit breaker.
// maxConns limits the number of concurrent connections.
func PoolDialer(address string, timeout time.Duration, maxConns int) Dialer {
	const failureThreshold = 3
	const resetTimeout = 30 * time.Second

	var pool *milter.Pool
	var initErr error

	return func(ctx context.Context) (Client, error) {
		// Lazy initialization of the pool
		if pool == nil && initErr == nil {
			var err error
			pool, err = milter.NewPoolWithCircuitBreaker("tcp", address, timeout, maxConns, failureThreshold, resetTimeout)
			if err != nil {
				initErr = err
				return nil, fmt.Errorf("milter pool init: %w", err)
			}
		}
		if initErr != nil {
			return nil, fmt.Errorf("milter pool init: %w", initErr)
		}

		c, err := pool.Get(ctx)
		if err != nil {
			return nil, err
		}

		// Return a wrapper that puts the client back to the pool on close
		return &pooledClient{Client: c, pool: pool}, nil
	}
}

// pooledClient wraps a milter.Client to return it to the pool on Close.
// It's used internally by PoolDialer to manage client lifetime.
type pooledClient struct {
	*milter.Client
	pool *milter.Pool
}

// Close returns the client to the pool instead of closing it.
func (pc *pooledClient) Close() error {
	if pc.Client != nil && pc.pool != nil {
		pc.pool.Put(pc.Client)
	}
	return nil
}

// HookOptions configures the milter hook.
type HookOptions struct {
	Dialer     Dialer
	ShadowMode bool // If true, log verdicts but don't enforce rejection/tempfail
}

// Hook returns a smtpd.Hook that runs the milter filter at StageParsed.
// Returns nil (disabled) when opts.Dialer is nil.
func Hook(opts HookOptions) smtpd.Hook {
	return func(ctx context.Context, event smtpd.Event) error {
		if event.Stage != smtpd.StageParsed || opts.Dialer == nil {
			return nil
		}
		return runMilter(ctx, event, opts.Dialer, opts.ShadowMode)
	}
}

func runMilter(ctx context.Context, event smtpd.Event, dial Dialer, shadowMode bool) error {
	client, err := dial(ctx)
	if err != nil {
		return fmt.Errorf("milter: dial: %w", err)
	}
	defer client.Close()

	if err := client.Negotiate(ctx); err != nil {
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

	if action, err := client.Connect(ctx, host, family, port, host); err != nil {
		return fmt.Errorf("milter: connect: %w", err)
	} else if err := verdictError(action, shadowMode); err != nil {
		return err
	}

	if action, err := client.MailFrom(ctx, event.EnvelopeFrom); err != nil {
		return fmt.Errorf("milter: mail from: %w", err)
	} else if err := verdictError(action, shadowMode); err != nil {
		return err
	}

	for _, rcpt := range event.Recipients {
		if action, err := client.RcptTo(ctx, rcpt); err != nil {
			return fmt.Errorf("milter: rcpt to: %w", err)
		} else if err := verdictError(action, shadowMode); err != nil {
			return err
		}
	}

	for _, h := range synthesizeHeaders(event) {
		if action, err := client.Header(ctx, h[0], h[1]); err != nil {
			return fmt.Errorf("milter: header: %w", err)
		} else if err := verdictError(action, shadowMode); err != nil {
			return err
		}
	}

	if action, err := client.EndOfHeaders(ctx); err != nil {
		return fmt.Errorf("milter: eoh: %w", err)
	} else if err := verdictError(action, shadowMode); err != nil {
		return err
	}

	if event.Parsed.TextBody != "" {
		if action, err := client.BodyChunk(ctx, []byte(event.Parsed.TextBody)); err != nil {
			return fmt.Errorf("milter: body: %w", err)
		} else if err := verdictError(action, shadowMode); err != nil {
			return err
		}
	}

	action, err := client.EndOfMessage(ctx)
	if err != nil {
		return fmt.Errorf("milter: eom: %w", err)
	}
	if err := verdictError(action, shadowMode); err != nil {
		return err
	}

	_ = client.Quit(ctx)
	return nil
}

func verdictError(action milter.Action, shadowMode bool) error {
	switch action {
	case milter.ActionReject:
		if shadowMode {
			return nil // Log but don't reject in shadow mode
		}
		return fmt.Errorf("milter: message rejected")
	case milter.ActionTempfail:
		if shadowMode {
			return nil // Log but don't reject in shadow mode
		}
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
