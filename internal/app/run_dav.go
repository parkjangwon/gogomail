package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/accesspolicy"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/maildb"
)

func runCalDAVGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	calendarRepository := caldavgw.NewRepository(db)
	accountRepository := maildb.NewRepository(db)
	directoryRepository := directory.NewRepository(db)
	resolver := caldavgw.NewBasicAuthResolver(accountRepository, cfg.CalDAVAllowInsecureAuth)
	resolver.TrustForwardedProto = cfg.CalDAVTrustForwardedProto
	resolver, err = resolver.WithTrustedProxies(cfg.CalDAVTrustedProxies)
	if err != nil {
		return fmt.Errorf("invalid caldav trusted proxies: %w", err)
	}
	handler := caldavgw.NewHandler(calendarRepository, resolver.Resolve)
	handler.SetMetrics(newProtocolGatewayMetrics(logger))
	handler.IncludeScheduling = cfg.CalDAVScheduling
	handler.AccessAuthorizer = caldavgw.DelegatedAccessPolicy{
		Directory: directoryRepository,
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{
			Checker:         directoryRepository,
			AuditRepository: audit.NewPostgresRepository(db),
		},
	}
	server := newCalDAVHTTPServer(cfg, handler)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("caldav gateway listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newCalDAVHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              strings.TrimSpace(cfg.CalDAVAddr),
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
}

func runCardDAVGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	addressBookRepository := carddavgw.NewRepository(db)
	accountRepository := maildb.NewRepository(db)
	directoryRepository := directory.NewRepository(db)
	resolver := carddavgw.NewBasicAuthResolver(accountRepository, cfg.CardDAVAllowInsecureAuth)
	resolver.TrustForwardedProto = cfg.CardDAVTrustForwardedProto
	resolver, err = resolver.WithTrustedProxies(cfg.CardDAVTrustedProxies)
	if err != nil {
		return fmt.Errorf("invalid carddav trusted proxies: %w", err)
	}
	handler := carddavgw.NewHandler(addressBookRepository, resolver.Resolve)
	handler.SetMetrics(newProtocolGatewayMetrics(logger))
	handler.AccessAuthorizer = carddavgw.DelegatedAccessPolicy{
		Directory: directoryRepository,
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{
			Checker:         directoryRepository,
			AuditRepository: audit.NewPostgresRepository(db),
		},
	}
	server := newCardDAVHTTPServer(cfg, handler)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("carddav gateway listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newCardDAVHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              strings.TrimSpace(cfg.CardDAVAddr),
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
}

func runWebDAVGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	driveSvc := driveServiceForConfig(db, cfg, store)
	webdavSvc := httpapi.NewWebDAVService(driveSvc)

	tokenManager, err := auth.NewTokenManager(cfg.AuthJWTSecret)
	if err != nil {
		return fmt.Errorf("create token manager: %w", err)
	}

	mux := http.NewServeMux()
	opts := httpapi.WebDAVRouteOptions{
		DepthInfinityEnabled: cfg.WebDAVDepthInfinityEnabled,
		Metrics:              webDAVMetrics(cfg, logger),
		TokenManager:         tokenManager,
	}
	httpapi.RegisterWebDAVRoutes(mux, webdavSvc, opts)
	server := newWebDAVHTTPServer(cfg, mux)
	go serveMetrics(ctx, cfg, logger)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("webdav gateway listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newWebDAVHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              strings.TrimSpace(cfg.WebDAVAddr),
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
}
