package smtpd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/mail"
)

type ServerOptions struct {
	Addr     string
	Domain   string
	Receiver *Receiver
	Logger   *slog.Logger
}

func RunServer(ctx context.Context, opts ServerOptions) error {
	if opts.Receiver == nil {
		return fmt.Errorf("smtp receiver is required")
	}
	if strings.TrimSpace(opts.Addr) == "" {
		return fmt.Errorf("smtp listen address is required")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	server := gosmtp.NewServer(opts.Receiver)
	server.Addr = opts.Addr
	server.Domain = opts.Domain
	server.ReadTimeout = 30 * time.Second
	server.WriteTimeout = 30 * time.Second
	server.MaxMessageBytes = 25 * 1024 * 1024
	server.MaxRecipients = 100
	server.AllowInsecureAuth = true

	errCh := make(chan error, 1)
	go func() {
		logger.Info("smtp server listening", "addr", opts.Addr, "domain", opts.Domain)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}

func StaticResolverFromRecipients(recipients []string) (StaticResolver, error) {
	resolver := make(StaticResolver, len(recipients))
	for _, recipient := range recipients {
		normalized, err := mail.NormalizeAddress(recipient)
		if err != nil {
			return nil, err
		}
		local, domain, _ := strings.Cut(normalized, "@")
		resolver[normalized] = Mailbox{
			CompanyID: "local",
			DomainID:  domain,
			UserID:    local,
			Address:   normalized,
		}
	}
	return resolver, nil
}
