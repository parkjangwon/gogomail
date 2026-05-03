package smtpd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/mail"
)

type ServerOptions struct {
	Addr              string
	Domain            string
	Backend           gosmtp.Backend
	Receiver          *Receiver
	Logger            *slog.Logger
	TLSConfig         *tls.Config
	AllowInsecureAuth bool
	EnableSMTPUTF8    bool
}

func RunServer(ctx context.Context, opts ServerOptions) error {
	backend := opts.Backend
	if backend == nil {
		backend = opts.Receiver
	}
	if backend == nil {
		return fmt.Errorf("smtp backend is required")
	}
	if strings.TrimSpace(opts.Addr) == "" {
		return fmt.Errorf("smtp listen address is required")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	server := gosmtp.NewServer(backend)
	server.Addr = opts.Addr
	server.Domain = opts.Domain
	server.TLSConfig = opts.TLSConfig
	server.ReadTimeout = 30 * time.Second
	server.WriteTimeout = 30 * time.Second
	server.MaxMessageBytes = 25 * 1024 * 1024
	server.MaxRecipients = 100
	server.AllowInsecureAuth = opts.AllowInsecureAuth
	server.EnableSMTPUTF8 = opts.EnableSMTPUTF8

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
