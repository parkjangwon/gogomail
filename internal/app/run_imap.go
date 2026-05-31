package app

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/deltasync"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/imapnotify"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

type imapGatewayRuntime struct {
	service *mailservice.Service
	store   mailservice.IMAPStoreAdapter
	backend mailservice.IMAPBackendAdapter
	events  *imapgw.MailboxEventBroker
	fanOut  *deltasync.FanOut
}

// fanOutAdapter implements imapnotify.DeltaSyncNotifier using deltasync.FanOut.
type fanOutAdapter struct {
	fanOut *deltasync.FanOut
}

func (a *fanOutAdapter) NotifyMailboxChange(mailboxID string, version int64) {
	a.fanOut.Notify(mailboxID, deltasync.Event{
		MailboxID: mailboxID,
		Type:      "mail.stored",
		Version:   version,
	})
}

type imapServerOptions struct {
	Addr              string
	Backend           imapgw.Backend
	TLSConfig         *tls.Config
	AllowInsecureAuth bool
	MaxConnections    int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func newIMAPGatewayRuntime(cfg config.Config, repository mailservice.Repository, store storage.Store, authenticator smtpd.SubmissionAuthenticator, quotaAlertEmitter maildb.QuotaWarningEmitterInterface) imapGatewayRuntime {
	events := imapgw.NewMailboxEventBroker(32)
	fanOut := deltasync.NewFanOut()
	service := mailservice.New(repository, store).
		WithMessageBodyCache(cfg.MessageBodyCacheEntries, cfg.MessageBodyCacheTTL).
		WithIMAPMailboxEvents(events)
	if quotaAlertEmitter != nil {
		service = service.WithQuotaAlertEmitter(quotaAlertEmitter)
	}
	return imapGatewayRuntime{
		service: service,
		store:   mailservice.NewIMAPStoreAdapter(service),
		backend: mailservice.NewIMAPBackendAdapter(authenticator, service),
		events:  events,
		fanOut:  fanOut,
	}
}

func imapServerOptionsForConfig(cfg config.Config, backend imapgw.Backend) (imapServerOptions, error) {
	tlsConfig, err := imapTLSConfig(cfg)
	if err != nil {
		return imapServerOptions{}, err
	}
	return imapServerOptions{
		Addr:              strings.TrimSpace(cfg.IMAPAddr),
		Backend:           backend,
		TLSConfig:         tlsConfig,
		AllowInsecureAuth: cfg.IMAPAllowInsecureAuth,
		MaxConnections:    cfg.IMAPMaxConnections,
		ReadTimeout:       cfg.IMAPReadTimeout,
		WriteTimeout:      cfg.IMAPWriteTimeout,
		IdleTimeout:       cfg.IMAPIdleTimeout,
	}, nil
}

func newIMAPServer(opts imapServerOptions) (*imapgw.Server, error) {
	return imapgw.NewServer(imapgw.ServerOptions{
		Addr:              opts.Addr,
		Backend:           opts.Backend,
		TLSConfig:         opts.TLSConfig,
		AllowInsecureAuth: opts.AllowInsecureAuth,
		MaxConnections:    opts.MaxConnections,
		ReadTimeout:       opts.ReadTimeout,
		WriteTimeout:      opts.WriteTimeout,
		IdleTimeout:       opts.IdleTimeout,
	})
}

func newIMAPMailboxEventRouter(uidEnsurer imapnotify.UIDEnsurer, events imapnotify.MailboxEventPublisher, deltaSync imapnotify.DeltaSyncNotifier) (*eventstream.Router, error) {
	router := eventstream.NewRouter()
	handler := imapnotify.NewMailStoredHandler(uidEnsurer).WithMailboxEvents(events)
	if deltaSync != nil {
		handler = handler.WithDeltaSync(deltaSync)
	}
	if err := router.Register("mail.stored", handler); err != nil {
		return nil, err
	}
	return router, nil
}

func runIMAPGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

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
	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(runCtx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()
	quotaAlertEmitter := maildb.NewQuotaWarningEmitter(db, redisClient, cfg.EventStream)
	runtime := newIMAPGatewayRuntime(cfg, repository, store, repository, quotaAlertEmitter)
	searchIDSource, err := searchIDSourceForConfig(cfg)
	if err != nil {
		return err
	}
	if searchIDSource != nil {
		runtime.service.WithSearchIDSource(searchIDSource)
	}

	router, err := newIMAPMailboxEventRouter(repository, runtime.events, &fanOutAdapter{fanOut: runtime.fanOut})
	if err != nil {
		return err
	}
	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.IMAPNotifyConsumerGroup,
		Consumer:         cfg.IMAPNotifyConsumerName,
		Count:            int64(cfg.IMAPNotifyConsumerCount),
		Block:            cfg.IMAPNotifyConsumerBlock,
		ClaimIdle:        cfg.IMAPNotifyConsumerClaimIdle,
		MaxDeliveries:    cfg.IMAPNotifyConsumerMaxDeliveries,
		DeadLetterStream: cfg.IMAPNotifyConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
	})
	if err != nil {
		return err
	}

	serverOptions, err := imapServerOptionsForConfig(cfg, runtime.backend)
	if err != nil {
		return err
	}
	server, err := newIMAPServer(serverOptions)
	if err != nil {
		return err
	}
	server.SetMetrics(newProtocolGatewayMetrics(logger))
	listener, err := server.Listen()
	if err != nil {
		return err
	}
	errCh := make(chan error, 2)
	go func() {
		logger.Info(
			"imap server listening",
			"mode", ModeIMAP,
			"addr", listener.Addr().String(),
			"tls_configured", serverOptions.TLSConfig != nil,
			"allow_insecure_auth", serverOptions.AllowInsecureAuth,
			"mailbox_event_broker", runtime.events != nil,
			"backend_adapter", "service",
		)
		errCh <- server.Serve(listener)
	}()
	go func() {
		logger.Info(
			"imap mailbox notification consumer started",
			"stream", cfg.EventStream,
			"group", cfg.IMAPNotifyConsumerGroup,
			"consumer", cfg.IMAPNotifyConsumerName,
			"count", cfg.IMAPNotifyConsumerCount,
			"block", cfg.IMAPNotifyConsumerBlock.String(),
			"max_deliveries", cfg.IMAPNotifyConsumerMaxDeliveries,
			"dead_letter_stream", cfg.IMAPNotifyConsumerDeadLetterStream,
		)
		errCh <- consumer.Run(runCtx)
	}()

	select {
	case <-ctx.Done():
		cancel()
		if err := listener.Close(); err != nil {
			logger.Warn("close listener", "error", err)
		}
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		for range 2 {
			err := <-errCh
			if err != nil && !errors.Is(err, imapgw.ErrServerClosed) {
				return err
			}
		}
		return nil
	case err := <-errCh:
		cancel()
		if err := listener.Close(); err != nil {
			logger.Warn("close listener", "error", err)
		}
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		otherErr := <-errCh
		if err == nil || errors.Is(err, imapgw.ErrServerClosed) {
			err = otherErr
		}
		if err != nil && !errors.Is(err, imapgw.ErrServerClosed) {
			return err
		}
		return nil
	}
}
