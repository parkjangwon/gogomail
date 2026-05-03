package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/maildb"
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
	var resolver smtpd.RecipientResolver
	var recorder smtpd.MessageRecorder

	if len(cfg.LocalRecipients) > 0 {
		staticResolver, err := smtpd.StaticResolverFromRecipients(cfg.LocalRecipients)
		if err != nil {
			return err
		}
		resolver = staticResolver
		logger.Info("edge-mta using static recipient resolver", "recipients", len(cfg.LocalRecipients))
	} else {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()

		repository := maildb.NewRepository(db)
		resolver = repository
		recorder = repository
		logger.Info("edge-mta using database recipient resolver and message recorder")
	}

	receiver := smtpd.NewReceiver(smtpd.ReceiverOptions{
		Store:    storage.NewLocalStore(cfg.MailstoreRoot),
		Resolver: resolver,
		Recorder: recorder,
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
