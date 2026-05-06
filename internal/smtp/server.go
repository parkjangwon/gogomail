package smtpd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
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
	ImplicitTLS       bool
	MaxConnections    int
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
	if opts.ImplicitTLS && opts.TLSConfig == nil {
		return fmt.Errorf("implicit TLS SMTP listener requires TLS configuration")
	}
	if opts.ImplicitTLS && !hasServerCertificate(opts.TLSConfig) {
		return fmt.Errorf("implicit TLS SMTP listener requires a server certificate")
	}
	if opts.MaxConnections < 0 {
		return fmt.Errorf("smtp max connections must not be negative")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	server := newSMTPServer(backend, opts)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("smtp server listening", "addr", opts.Addr, "domain", opts.Domain, "implicit_tls", opts.ImplicitTLS)
		errCh <- listenAndServeSMTP(server, opts.ImplicitTLS, opts.MaxConnections)
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
	server.TLSConfig = normalizeServerTLSConfig(opts.TLSConfig)
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

func listenAndServeSMTP(server *gosmtp.Server, implicitTLS bool, maxConnections int) error {
	network := "tcp"
	addr := server.Addr
	if strings.TrimSpace(addr) == "" {
		if implicitTLS {
			addr = ":smtps"
		} else {
			addr = ":smtp"
		}
	}
	var listener net.Listener
	var err error
	if implicitTLS {
		listener, err = tls.Listen(network, addr, server.TLSConfig)
	} else {
		listener, err = net.Listen(network, addr)
	}
	if err != nil {
		return err
	}
	if maxConnections > 0 {
		listener = newSMTPConnectionLimitListener(listener, maxConnections)
	}
	return server.Serve(listener)
}

type smtpConnectionLimitListener struct {
	net.Listener
	slots chan struct{}
}

func newSMTPConnectionLimitListener(listener net.Listener, maxConnections int) net.Listener {
	if maxConnections <= 0 {
		return listener
	}
	return &smtpConnectionLimitListener{
		Listener: listener,
		slots:    make(chan struct{}, maxConnections),
	}
}

func (l *smtpConnectionLimitListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.slots <- struct{}{}:
			return &smtpConnectionLimitConn{Conn: conn, release: func() { <-l.slots }}, nil
		default:
			rejectSMTPConnectionOverLimit(conn)
		}
	}
}

type smtpConnectionLimitConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *smtpConnectionLimitConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}

func rejectSMTPConnectionOverLimit(conn net.Conn) {
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, _ = io.WriteString(conn, "421 4.3.2 Too many connections, try again later\r\n")
	_ = conn.Close()
}

func normalizeServerTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return nil
	}
	normalized := cfg.Clone()
	if normalized.MinVersion == 0 || normalized.MinVersion < tls.VersionTLS12 {
		normalized.MinVersion = tls.VersionTLS12
	}
	return normalized
}

func hasServerCertificate(cfg *tls.Config) bool {
	if cfg == nil {
		return false
	}
	return len(cfg.Certificates) > 0 || cfg.GetCertificate != nil || cfg.GetConfigForClient != nil
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
