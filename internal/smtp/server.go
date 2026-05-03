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
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	MaxMessageBytes   int64
	MaxRecipients     int
	AllowInsecureAuth bool
	EnableSMTPUTF8    bool
	EnableRequireTLS  bool
	EnableDSN         bool
	EnableBinaryMIME  bool
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

	server := newSMTPServer(backend, opts)

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

func newSMTPServer(backend gosmtp.Backend, opts ServerOptions) *gosmtp.Server {
	server := gosmtp.NewServer(backend)
	server.Addr = opts.Addr
	server.Domain = opts.Domain
	server.TLSConfig = opts.TLSConfig
	server.ReadTimeout = durationOrDefault(opts.ReadTimeout, 30*time.Second)
	server.WriteTimeout = durationOrDefault(opts.WriteTimeout, 30*time.Second)
	server.MaxMessageBytes = int64OrDefault(opts.MaxMessageBytes, 25*1024*1024)
	server.MaxRecipients = intOrDefault(opts.MaxRecipients, 100)
	server.AllowInsecureAuth = opts.AllowInsecureAuth
	server.EnableSMTPUTF8 = opts.EnableSMTPUTF8
	server.EnableREQUIRETLS = opts.EnableRequireTLS
	server.EnableDSN = opts.EnableDSN
	server.EnableBINARYMIME = opts.EnableBinaryMIME
	return server
}

func durationOrDefault(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func int64OrDefault(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func intOrDefault(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
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
