package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/httpapi"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

func Run(ctx context.Context, mode Mode, cfg config.Config, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("starting gogomail", "mode", mode, "env", cfg.Environment)

	switch mode {
	case ModeAllInOne, ModeAuthServer, ModeMailAPI, ModeAdminAPI:
		return runHTTP(ctx, cfg, logger, mode)
	case ModeEdgeMTA:
		return runEdgeMTA(ctx, cfg, logger)
	case ModeInboundMTA, ModeOutboundMTA, ModeDeliveryWorker, ModeBatchWorker, ModeOutboxRelay:
		return waitForShutdown(ctx, logger, mode)
	default:
		return errors.New("unsupported mode")
	}
}

func runEdgeMTA(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	resolver, err := smtpd.StaticResolverFromRecipients(cfg.LocalRecipients)
	if err != nil {
		return err
	}
	if len(resolver) == 0 {
		logger.Warn("edge-mta has no local recipients configured; all RCPT commands will be rejected")
	}

	receiver := smtpd.NewReceiver(smtpd.ReceiverOptions{
		Store:    storage.NewLocalStore(cfg.MailstoreRoot),
		Resolver: resolver,
	})

	return smtpd.RunServer(ctx, smtpd.ServerOptions{
		Addr:     cfg.SMTPAddr,
		Domain:   cfg.SMTPDomain,
		Receiver: receiver,
		Logger:   logger,
	})
}

func runHTTP(ctx context.Context, cfg config.Config, logger *slog.Logger, mode Mode) error {
	mux := http.NewServeMux()
	httpapi.RegisterHealthRoutes(mux)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr, "mode", mode)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func waitForShutdown(ctx context.Context, logger *slog.Logger, mode Mode) error {
	logger.Info("mode scaffold is ready; component implementation will be added next", "mode", mode)
	<-ctx.Done()
	return nil
}
