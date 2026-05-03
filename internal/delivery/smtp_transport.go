package delivery

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"sort"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

type MXResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
}

type DeliveryTLSMode string

const (
	DeliveryTLSOpportunistic DeliveryTLSMode = "opportunistic"
	DeliveryTLSRequire       DeliveryTLSMode = "require"
	DeliveryTLSDisable       DeliveryTLSMode = "disable"
)

type DirectSMTPTransport struct {
	Resolver     MXResolver
	Router       Router
	Timeout      time.Duration
	Hello        string
	TLSMode      DeliveryTLSMode
	TLSConfig    *tls.Config
	Transformers TransformChain
}

func NewDirectSMTPTransport() *DirectSMTPTransport {
	return &DirectSMTPTransport{
		Resolver: net.DefaultResolver,
		Timeout:  30 * time.Second,
		Hello:    "localhost",
		TLSMode:  DeliveryTLSOpportunistic,
	}
}

func (t *DirectSMTPTransport) Deliver(ctx context.Context, job Job) error {
	groups := groupRecipientsByDomain(job.Recipients())
	for domain, recipients := range groups {
		if err := t.deliverDomain(ctx, job, domain, recipients); err != nil {
			return err
		}
	}
	return nil
}

func (t *DirectSMTPTransport) deliverDomain(ctx context.Context, job Job, domain string, recipients []outbound.Address) error {
	route, err := t.route(ctx, job, domain)
	if err != nil {
		return err
	}
	hosts := route.Hosts
	if len(hosts) == 0 {
		hosts, err = t.mxHosts(ctx, domain)
		if err != nil {
			return err
		}
		route.Hosts = hosts
	}
	errs := make([]error, 0, len(hosts))
	for _, host := range hosts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := t.deliverHost(ctx, job, route, host, recipients); err != nil {
			if IsPermanentFailure(err) {
				return err
			}
			errs = append(errs, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("deliver to %s via %d mx host(s): %w", domain, len(hosts), errors.Join(errs...))
}

func (t *DirectSMTPTransport) deliverHost(ctx context.Context, job Job, route Route, host string, recipients []outbound.Address) error {
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "25"))
	if err != nil {
		return fmt.Errorf("dial mx %s for %s: %w", host, route.Domain, err)
	}
	defer conn.Close()
	if deadline := deliveryDeadline(ctx, timeout, time.Now()); !deadline.IsZero() {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set smtp session deadline for %s: %w", host, err)
		}
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("create smtp client for %s: %w", host, err)
	}
	defer client.Close()

	hello := strings.TrimSpace(t.Hello)
	if route.Hello != "" {
		hello = route.Hello
	}
	if hello == "" {
		hello = "localhost"
	}
	if err := client.Hello(hello); err != nil {
		return WrapSMTPError("hello", err)
	}
	if err := t.startTLS(ctx, client, host, route.TLSMode); err != nil {
		return WrapSMTPError("starttls", err)
	}
	if err := client.Mail(job.From.Email); err != nil {
		return WrapSMTPError("mail", err)
	}
	acceptedRecipients, rcptErr := acceptRecipients(recipients, func(recipient outbound.Address) error {
		if err := client.Rcpt(recipient.Email); err != nil {
			return WrapSMTPError("rcpt", err)
		}
		return nil
	})
	if len(acceptedRecipients) == 0 {
		return rcptErr
	}

	writer, err := client.Data()
	if err != nil {
		return WrapSMTPError("data", err)
	}
	message, err := t.openMessage(ctx, job)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("open queued message: %w", err)
	}
	_, copyErr := io.Copy(writer, message)
	closeMessageErr := message.Close()
	closeDataErr := writer.Close()
	if copyErr != nil {
		return fmt.Errorf("write smtp data: %w", copyErr)
	}
	if closeMessageErr != nil {
		return fmt.Errorf("close queued message: %w", closeMessageErr)
	}
	if closeDataErr != nil {
		return WrapSMTPError("data", closeDataErr)
	}
	if err := client.Quit(); err != nil {
		return WrapSMTPError("quit", err)
	}
	return nil
}

func acceptRecipients(recipients []outbound.Address, rcpt func(outbound.Address) error) ([]outbound.Address, error) {
	accepted := make([]outbound.Address, 0, len(recipients))
	errs := make([]error, 0)
	for _, recipient := range recipients {
		if err := rcpt(recipient); err != nil {
			errs = append(errs, fmt.Errorf("recipient %s: %w", recipient.Email, err))
			continue
		}
		accepted = append(accepted, recipient)
	}
	return accepted, errors.Join(errs...)
}

func (t *DirectSMTPTransport) startTLS(ctx context.Context, client *smtp.Client, host string, modeOverride DeliveryTLSMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mode := normalizeDeliveryTLSMode(t.TLSMode)
	if modeOverride != "" {
		mode = normalizeDeliveryTLSMode(modeOverride)
	}
	if mode == DeliveryTLSDisable {
		return nil
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		if mode == DeliveryTLSRequire {
			return fmt.Errorf("STARTTLS is required but not advertised by %s", host)
		}
		return nil
	}
	return client.StartTLS(t.deliveryTLSConfig(host))
}

func (t *DirectSMTPTransport) deliveryTLSConfig(host string) *tls.Config {
	var cfg *tls.Config
	if t.TLSConfig != nil {
		cfg = t.TLSConfig.Clone()
	} else {
		cfg = &tls.Config{}
	}
	if strings.TrimSpace(cfg.ServerName) == "" {
		cfg.ServerName = strings.TrimSpace(host)
	}
	if cfg.MinVersion == 0 || cfg.MinVersion < tls.VersionTLS12 {
		cfg.MinVersion = tls.VersionTLS12
	}
	return cfg
}

func (t *DirectSMTPTransport) route(ctx context.Context, job Job, domain string) (Route, error) {
	if t.Router == nil {
		return normalizeRoute(job, domain, Route{TLSMode: t.TLSMode}), nil
	}
	route, err := t.Router.Route(ctx, job, domain)
	if err != nil {
		return Route{}, fmt.Errorf("route delivery for %s: %w", domain, err)
	}
	return normalizeRoute(job, domain, route), nil
}

func (t *DirectSMTPTransport) openMessage(ctx context.Context, job Job) (io.ReadCloser, error) {
	message, err := job.OpenMessage(ctx)
	if err != nil {
		return nil, err
	}
	if len(t.Transformers) == 0 {
		return message, nil
	}
	return t.Transformers.Transform(ctx, job, message)
}

func (t *DirectSMTPTransport) mxHosts(ctx context.Context, domain string) ([]string, error) {
	resolver := t.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	records, err := resolver.LookupMX(ctx, domain)
	if err != nil || len(records) == 0 {
		return []string{domain}, nil
	}
	if isNullMX(records) {
		return nil, &SMTPStatusError{
			Op:      "mx",
			Code:    556,
			Message: fmt.Sprintf("domain %s publishes null MX and does not accept mail", domain),
		}
	}
	hosts := orderedMXHosts(records)
	if len(hosts) == 0 {
		return []string{domain}, nil
	}
	return hosts, nil
}

func orderedMXHosts(records []*net.MX) []string {
	ordered := make([]*net.MX, 0, len(records))
	for _, record := range records {
		if record != nil {
			ordered = append(ordered, record)
		}
	}
	hosts := make([]string, 0, len(ordered))
	if len(ordered) == 0 {
		return hosts
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Pref == ordered[j].Pref {
			return ordered[i].Host < ordered[j].Host
		}
		return ordered[i].Pref < ordered[j].Pref
	})
	for _, record := range ordered {
		host := strings.TrimSpace(strings.TrimSuffix(record.Host, "."))
		if host == "" || host == "." {
			continue
		}
		hosts = append(hosts, host)
	}
	return hosts
}

func isNullMX(records []*net.MX) bool {
	return len(records) == 1 && records[0] != nil && records[0].Pref == 0 && strings.TrimSpace(records[0].Host) == "."
}

func deliveryDeadline(ctx context.Context, timeout time.Duration, now time.Time) time.Time {
	var deadline time.Time
	if timeout > 0 {
		deadline = now.Add(timeout)
	}
	if ctxDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || ctxDeadline.Before(deadline)) {
		deadline = ctxDeadline
	}
	return deadline
}

func normalizeDeliveryTLSMode(mode DeliveryTLSMode) DeliveryTLSMode {
	switch DeliveryTLSMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case DeliveryTLSRequire:
		return DeliveryTLSRequire
	case DeliveryTLSDisable:
		return DeliveryTLSDisable
	default:
		return DeliveryTLSOpportunistic
	}
}

func groupRecipientsByDomain(recipients []outbound.Address) map[string][]outbound.Address {
	groups := make(map[string][]outbound.Address)
	for _, recipient := range recipients {
		_, domain, ok := strings.Cut(strings.TrimSpace(recipient.Email), "@")
		if !ok || domain == "" {
			continue
		}
		groups[strings.ToLower(domain)] = append(groups[strings.ToLower(domain)], recipient)
	}
	return groups
}
