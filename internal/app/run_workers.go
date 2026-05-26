package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/directory"
	dsnpkg "github.com/gogomail/gogomail/internal/dsn"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/inboundfilter"
	"github.com/gogomail/gogomail/internal/imapnotify"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailflow"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/observability"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/outbox"
	"github.com/gogomail/gogomail/internal/scheduling"
	"github.com/gogomail/gogomail/internal/searchindex"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
)

func runOutboxRelay(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	tp, err := observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:          cfg.OTelEnabled,
		ExporterEndpoint: cfg.OTelEndpoint,
		ServiceName:      cfg.OTelServiceName + "-outbox",
		ServiceVersion:   cfg.OTelServiceVersion,
	})
	if err != nil {
		logger.Warn("tracing init failed in outbox relay", "error", err)
	} else {
		defer func() { _ = tp.Shutdown(context.Background()) }()
	}

	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()

	// Choose store: sharded (for partition-ordered multi-process scaling) or plain.
	var store outbox.Store
	if cfg.OutboxRelayShardTotal > 1 {
		sharded, err := outbox.NewShardedPostgresStore(db, cfg.OutboxRelayMaxAttempts, cfg.OutboxRelayShardTotal, cfg.OutboxRelayShardIndex)
		if err != nil {
			return fmt.Errorf("create sharded outbox store: %w", err)
		}
		store = sharded
		logger.Info("outbox relay sharding enabled",
			"shard_index", cfg.OutboxRelayShardIndex,
			"shard_total", cfg.OutboxRelayShardTotal,
		)
	} else {
		store = outbox.NewPostgresStore(db, cfg.OutboxRelayMaxAttempts)
	}

	relay, err := outbox.NewRelay(outbox.RelayOptions{
		Store:        store,
		Publisher:    outbox.NewRedisStreamPublisher(redisClient, cfg.EventStream),
		BatchSize:    cfg.OutboxRelayBatchSize,
		PollInterval: cfg.OutboxRelayPollInterval,
		WorkerCount:  cfg.OutboxRelayWorkerCount,
		Logger:       logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"outbox relay started",
		"redis_addr", cfg.RedisAddr,
		"stream", cfg.EventStream,
		"batch_size", cfg.OutboxRelayBatchSize,
		"poll_interval", cfg.OutboxRelayPollInterval.String(),
		"max_attempts", cfg.OutboxRelayMaxAttempts,
		"workers", cfg.OutboxRelayWorkerCount,
	)
	return relay.Run(ctx)
}

func runEventWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()

	router := eventstream.NewRouter()
	auditRepository := audit.NewPostgresRepository(db)
	mailFlowHandler := mailflow.NewHandler(db)

	if cfg.MailFlowOpenSearchBootstrap && strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		opts := mailFlowOpenSearchOptionsForConfig(cfg)
		indexer, err := searchindex.NewMailFlowIndexer(opts)
		if err != nil {
			return fmt.Errorf("create mail flow indexer: %w", err)
		}
		if err := indexer.EnsureIndex(ctx); err != nil {
			return fmt.Errorf("bootstrap mail flow index: %w", err)
		}
		mailFlowHandler = mailflow.NewHandlerWithIndexer(db, &indexer)
	}
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	// Build webhook dispatcher early so it can be included in MultiHandlers below.
	var webhookStoredHandler, webhookDeliveredHandler, webhookBouncedHandler eventstream.Handler
	if cfg.WebhookDispatchEnabled {
		webhookConfigStore := configstore.NewPostgresConfigStore(db)
		webhookLoader := webhookguard.NewConfigStoreLoader(webhookConfigStore)
		webhookDispatcher := webhookguard.NewWebhookDispatcher(webhookguard.WebhookDispatcherOptions{
			Loader: webhookLoader,
			Logger: logger,
		})
		webhookStoredHandler = webhookguard.NewMailStoredWebhookHandler(webhookDispatcher)
		webhookDeliveredHandler = webhookguard.NewMailSentWebhookHandler(webhookDispatcher)
		webhookBouncedHandler = webhookguard.NewMailBouncedWebhookHandler(webhookDispatcher)
		logger.Info("webhook dispatcher enabled")
	}

	if err := router.Register("mail.stored", eventstream.MultiHandler{
		imapnotify.NewMailStoredHandler(maildb.NewRepository(db)),
		audit.NewMailStoredHandler(auditRepository),
		mailFlowHandler,
		inboundfilter.NewHandler(mailservice.New(maildb.NewRepository(db), store)),
		webhookStoredHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivered", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		mailFlowHandler,
		webhookDeliveredHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.bounced", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		dsnpkg.NewBounceHandler(dsnpkg.HandlerOptions{
			Store:        store,
			Queue:        dsnpkg.NewPostgresOutboxQueue(db),
			ReportingMTA: cfg.SMTPDomain,
			Postmaster:   cfg.DSNPostmaster,
			Farm:         outbound.FarmGeneral,
		}),
		mailFlowHandler,
		webhookBouncedHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivery_failed", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		mailFlowHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivery_exhausted", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		dsnpkg.NewBounceHandler(dsnpkg.HandlerOptions{
			Store:        store,
			Queue:        dsnpkg.NewPostgresOutboxQueue(db),
			ReportingMTA: cfg.SMTPDomain,
			Postmaster:   cfg.DSNPostmaster,
			Farm:         outbound.FarmGeneral,
		}),
		mailFlowHandler,
	}); err != nil {
		return err
	}
	davAuditHandler := audit.NewDAVChangeHandler(auditRepository)
	if err := router.Register(audit.DAVChangeEventCalendar, davAuditHandler); err != nil {
		return err
	}
	if err := router.Register(audit.DAVChangeEventContacts, davAuditHandler); err != nil {
		return err
	}

	schedulingObjectStore, err := objectStoreForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create scheduling object store: %w", err)
	}
	schedulingQueue := scheduling.NewDeliveryQueue(redisClient)
	directoryRepo := directory.NewRepository(db)
	carddavRepo := carddavgw.NewRepository(db)
	attendeeResolver := scheduling.NewDefaultAttendeeResolver(directoryRepo, carddavRepo)
	schedulingHandler := scheduling.NewHandler(logger, schedulingQueue, schedulingObjectStore, attendeeResolver)
	if err := router.Register("scheduling.outbox", schedulingHandler); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.EventConsumerGroup,
		Consumer:         cfg.EventConsumerName,
		Count:            int64(cfg.EventConsumerCount),
		Block:            cfg.EventConsumerBlock,
		ClaimIdle:        cfg.EventConsumerClaimIdle,
		MaxDeliveries:    cfg.EventConsumerMaxDeliveries,
		DeadLetterStream: cfg.EventConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"event worker started",
		"stream", cfg.EventStream,
		"group", cfg.EventConsumerGroup,
		"consumer", cfg.EventConsumerName,
		"count", cfg.EventConsumerCount,
		"block", cfg.EventConsumerBlock.String(),
		"max_deliveries", cfg.EventConsumerMaxDeliveries,
		"dead_letter_stream", cfg.EventConsumerDeadLetterStream,
	)
	return consumer.Run(ctx)
}

