package delivery

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"sort"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

type DirectSMTPTransport struct {
	Resolver     *net.Resolver
	Timeout      time.Duration
	Hello        string
	Transformers TransformChain
}

func NewDirectSMTPTransport() *DirectSMTPTransport {
	return &DirectSMTPTransport{
		Resolver: net.DefaultResolver,
		Timeout:  30 * time.Second,
		Hello:    "localhost",
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
	host, err := t.mxHost(ctx, domain)
	if err != nil {
		return err
	}

	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "25"))
	if err != nil {
		return fmt.Errorf("dial mx %s for %s: %w", host, domain, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("create smtp client for %s: %w", host, err)
	}
	defer client.Close()

	hello := strings.TrimSpace(t.Hello)
	if hello == "" {
		hello = "localhost"
	}
	if err := client.Hello(hello); err != nil {
		return WrapSMTPError("hello", err)
	}
	if err := client.Mail(job.From.Email); err != nil {
		return WrapSMTPError("mail", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient.Email); err != nil {
			return WrapSMTPError("rcpt", err)
		}
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

func (t *DirectSMTPTransport) mxHost(ctx context.Context, domain string) (string, error) {
	resolver := t.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	records, err := resolver.LookupMX(ctx, domain)
	if err != nil || len(records) == 0 {
		return domain, nil
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Pref < records[j].Pref
	})
	return strings.TrimSuffix(records[0].Host, "."), nil
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
