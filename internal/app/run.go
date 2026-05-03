package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/dedup"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/outbox"
	"github.com/gogomail/gogomail/internal/ratelimit"
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
	case ModeOutboxRelay:
		return runOutboxRelay(ctx, cfg, logger)
	case ModeEventWorker:
		return runEventWorker(ctx, cfg, logger)
	case ModeDeliveryWorker:
		return runDeliveryWorker(ctx, cfg, logger)
	case ModeInboundMTA, ModeOutboundMTA, ModeBatchWorker:
		return waitForShutdown(ctx, logger, mode)
	default:
		return errors.New("unsupported mode")
	}
}

func runEdgeMTA(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	var resolver smtpd.RecipientResolver
	var recorder smtpd.MessageRecorder
	var deduplicator smtpd.Deduplicator
	var rateLimiter smtpd.RateLimiter
	var pressure smtpd.Backpressure
	var redisClient *redis.Client

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

	if cfg.DedupBackend == "redis" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			_ = redisClient.Close()
			return err
		}

		deduplicator = dedup.NewRedisDeduplicator(redisClient, 24*time.Hour)
		logger.Info("edge-mta using redis deduplicator", "addr", cfg.RedisAddr)
	}
	if cfg.RateLimitBackend == "redis" {
		if redisClient == nil {
			redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
			if err := redisClient.Ping(ctx).Err(); err != nil {
				_ = redisClient.Close()
				return err
			}
		}
		rateLimiter = ratelimit.NewRedisLimiter(redisClient, int64(cfg.RcptRateLimitPerMinute), time.Minute)
		logger.Info("edge-mta using redis rate limiter", "addr", cfg.RedisAddr, "rcpt_per_minute", cfg.RcptRateLimitPerMinute)
	}
	if cfg.BackpressureBackend == "redis" {
		if redisClient == nil {
			redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
			if err := redisClient.Ping(ctx).Err(); err != nil {
				_ = redisClient.Close()
				return err
			}
		}
		pressure = backpressure.NewRedisBackpressure(redisClient, backpressure.DefaultStateKey)
		logger.Info("edge-mta using redis backpressure", "addr", cfg.RedisAddr)
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	receiver := smtpd.NewReceiver(smtpd.ReceiverOptions{
		Store:        storage.NewLocalStore(cfg.MailstoreRoot),
		Resolver:     resolver,
		Recorder:     recorder,
		Deduplicator: deduplicator,
		RateLimiter:  rateLimiter,
		Backpressure: pressure,
	})

	return smtpd.RunServer(ctx, smtpd.ServerOptions{
		Addr:     cfg.SMTPAddr,
		Domain:   cfg.SMTPDomain,
		Receiver: receiver,
		Logger:   logger,
	})
}

func runOutboxRelay(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return err
	}
	defer redisClient.Close()

	relay, err := outbox.NewRelay(outbox.RelayOptions{
		Store:        outbox.NewPostgresStore(db, cfg.OutboxRelayMaxAttempts),
		Publisher:    outbox.NewRedisStreamPublisher(redisClient),
		BatchSize:    cfg.OutboxRelayBatchSize,
		PollInterval: cfg.OutboxRelayPollInterval,
		Logger:       logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"outbox relay started",
		"redis_addr", cfg.RedisAddr,
		"batch_size", cfg.OutboxRelayBatchSize,
		"poll_interval", cfg.OutboxRelayPollInterval.String(),
		"max_attempts", cfg.OutboxRelayMaxAttempts,
	)
	return relay.Run(ctx)
}

func runEventWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return err
	}
	defer redisClient.Close()

	router := eventstream.NewRouter()
	if err := router.Register("mail.stored", audit.NewMailStoredHandler(audit.NewPostgresRepository(db))); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:   redisClient,
		Stream:   cfg.EventStream,
		Group:    cfg.EventConsumerGroup,
		Consumer: cfg.EventConsumerName,
		Count:    int64(cfg.EventConsumerCount),
		Block:    cfg.EventConsumerBlock,
		Handler:  router,
		Logger:   logger,
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
	)
	return consumer.Run(ctx)
}

func runDeliveryWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return err
	}
	defer redisClient.Close()

	transport := delivery.NewDirectSMTPTransport()
	transport.Hello = cfg.DeliverySMTPHello

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:   redisClient,
		Stream:   cfg.DeliveryStream,
		Group:    cfg.DeliveryConsumerGroup,
		Consumer: cfg.DeliveryConsumerName,
		Count:    int64(cfg.DeliveryConsumerCount),
		Block:    cfg.DeliveryConsumerBlock,
		Handler:  delivery.NewHandler(storage.NewLocalStore(cfg.MailstoreRoot), transport),
		Logger:   logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"delivery worker started",
		"stream", cfg.DeliveryStream,
		"group", cfg.DeliveryConsumerGroup,
		"consumer", cfg.DeliveryConsumerName,
		"count", cfg.DeliveryConsumerCount,
		"block", cfg.DeliveryConsumerBlock.String(),
	)
	return consumer.Run(ctx)
}

func runHTTP(ctx context.Context, cfg config.Config, logger *slog.Logger, mode Mode) error {
	mux := http.NewServeMux()
	httpapi.RegisterHealthRoutes(mux)

	if mode == ModeMailAPI {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()

		repository := maildb.NewRepository(db)
		service := mailservice.New(repository, storage.NewLocalStore(cfg.MailstoreRoot))
		httpapi.RegisterMailRoutes(mux, service)
		logger.Info("mail api routes registered")
	}

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
