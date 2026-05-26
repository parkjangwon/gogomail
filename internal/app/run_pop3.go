package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/pop3d"
)

func pop3TLSConfig(cfg config.Config) (*tls.Config, error) {
	if cfg.POP3TLSCertFile == "" && cfg.POP3TLSKeyFile == "" {
		return nil, nil
	}
	if cfg.POP3TLSCertFile == "" || cfg.POP3TLSKeyFile == "" {
		return nil, errors.New("both POP3 TLS certificate and key files are required")
	}
	cert, err := tls.LoadX509KeyPair(cfg.POP3TLSCertFile, cfg.POP3TLSKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func runPOP3Gateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}

	repository := maildb.NewRepository(db)
	service := mailservice.New(repository, store).WithMessageBodyCache(cfg.MessageBodyCacheEntries, cfg.MessageBodyCacheTTL)

	server, err := pop3ServerForConfig(cfg, repository, service)
	if err != nil {
		return err
	}

	addr := strings.TrimSpace(cfg.POP3Addr)
	if addr == "" {
		addr = ":1110"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("pop3 listen %s: %w", addr, err)
	}

	logger.Info("pop3 server listening", "mode", ModePOP3, "addr", ln.Addr().String(), "tls_configured", server.TLSConfig != nil)

	errCh := make(chan error, 2)
	go func() { errCh <- server.Serve(ln) }()

	pop3sAddr := strings.TrimSpace(cfg.POP3SAddr)
	if pop3sAddr != "" {
		if server.TLSConfig == nil {
			server.Close()
			return errors.New("GOGOMAIL_POP3S_ADDR requires POP3 TLS certificate and key files")
		}
		pop3sLn, err := net.Listen("tcp", pop3sAddr)
		if err != nil {
			server.Close()
			return fmt.Errorf("pop3s listen %s: %w", pop3sAddr, err)
		}
		logger.Info("pop3s server listening", "mode", ModePOP3, "addr", pop3sLn.Addr().String())
		go func() { errCh <- server.Serve(tls.NewListener(pop3sLn, server.TLSConfig)) }()
	}

	select {
	case <-ctx.Done():
		server.Close()
		return nil
	case err := <-errCh:
		return err
	}
}

func pop3ServerForConfig(cfg config.Config, repository *maildb.Repository, service *mailservice.Service) (*pop3d.Server, error) {
	tlsConfig, err := pop3TLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &pop3d.Server{
		Store:          mailservice.NewPOP3StoreAdapter(repository, service),
		TLSConfig:      tlsConfig,
		Greeting:       "gogomail POP3 ready",
		IdleTimeout:    cfg.POP3IdleTimeout,
		MaxConnections: cfg.POP3MaxConnections,
	}, nil
}
