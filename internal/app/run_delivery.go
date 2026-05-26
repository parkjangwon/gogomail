package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/dkim"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/maildb"
)

func runDeliveryWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
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

	transport := delivery.NewDirectSMTPTransport()
	transport.Hello = cfg.DeliverySMTPHello
	transport.TLSReportDomain = cfg.SMTPDomain
	transport.Timeout = cfg.DeliveryTimeout
	transport.TLSMode = delivery.DeliveryTLSMode(cfg.DeliveryTLSMode)
	transport.MaxRecipientsPerBatch = cfg.DeliveryRecipientBatchSize
	repository := maildb.NewRepository(db)
	if strings.EqualFold(strings.TrimSpace(cfg.DeliveryRouteBackend), "postgres") {
		transport.Router = postgresDeliveryRouter{repository: repository, fallbackTLSMode: delivery.DeliveryTLSMode(cfg.DeliveryTLSMode)}
		logger.Info("delivery worker using postgres delivery routes")
	} else if router := deliveryRouterFromConfig(cfg); router != nil {
		transport.Router = router
		logger.Info("delivery worker using smart-host relay", "host", cfg.DeliverySmartHost, "port", cfg.DeliverySmartHostPort, "tls_mode", cfg.DeliverySmartHostTLSMode, "implicit_tls", cfg.DeliverySmartHostImplicitTLS, "auth_configured", strings.TrimSpace(cfg.DeliverySmartHostUsername) != "")
	}
	retryPolicy := delivery.DefaultRetryPolicy()
	retryPolicy.Delays = cfg.DeliveryRetryDelays
	retryPolicy.JitterRatio = cfg.DeliveryRetryJitterRatio
	retryPolicy.MaxDelay = cfg.DeliveryRetryMaxDelay
	if cfg.DKIMEnabled {
		transport.Transformers = append(transport.Transformers, dkim.Transformer{
			Signer: dkim.RFC6376Signer{KeyProvider: dkimKeyProvider{repository: repository}},
		})
		logger.Info("delivery worker enabled DKIM signing transformer")
	}
	deliveryRecorder := delivery.NewPostgresRecorder(db)
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	var deliveryTransport delivery.Transport = transport
	if cfg.DeliveryCircuitBreakerEnabled {
		cb := delivery.NewCircuitBreakerTransport(transport, cfg.DeliveryCircuitBreakerMax)
		cb.FailureThreshold = cfg.DeliveryCircuitBreakerThreshold
		cb.HalfOpenTimeout = cfg.DeliveryCircuitBreakerTimeout
		deliveryTransport = cb
		logger.Info("delivery circuit breaker enabled",
			"max", cfg.DeliveryCircuitBreakerMax,
			"threshold", cfg.DeliveryCircuitBreakerThreshold,
			"timeout", cfg.DeliveryCircuitBreakerTimeout,
		)
	}

	localDelivery := localDeliveryAdapter{repository: repository}
	handler := delivery.NewHandler(
		store,
		deliveryTransport,
		deliveryRecorder,
		delivery.NewPostgresRetryScheduler(db, retryPolicy),
	).WithExhaustionHook(deliveryRecorder).WithLocalDelivery(localDelivery, localDelivery)
	if cfg.DeliveryThrottleEnabled {
		throttlePolicy := delivery.ThrottlePolicy{
			FarmMaxConcurrent:   deliveryFarmLimits(cfg.DeliveryFarmConcurrency),
			DomainMaxConcurrent: cfg.DeliveryDomainConcurrency,
			DefaultConcurrent:   cfg.DeliveryDefaultConcurrency,
		}
		if strings.EqualFold(strings.TrimSpace(cfg.DeliveryThrottleBackend), "redis") {
			handler.WithThrottler(delivery.NewCoordinatedThrottler(
				throttlePolicy,
				delivery.NewRedisThrottleCounter(redisClient, "gogomail:delivery:throttle"),
			))
		} else {
			handler.WithThrottler(delivery.NewInMemoryThrottler(throttlePolicy))
		}
		logger.Info(
			"delivery throttling enabled",
			"backend", cfg.DeliveryThrottleBackend,
			"default_concurrency", cfg.DeliveryDefaultConcurrency,
			"farm_limits", cfg.DeliveryFarmConcurrency,
			"domain_limits", cfg.DeliveryDomainConcurrency,
		)
	}
	if cfg.DeliveryRateLimitEnabled {
		rateLimitPolicy := delivery.DomainRateLimitPolicy{
			DomainMessagesPerMinute:  cfg.DeliveryDomainRateLimitPerMinute,
			DefaultMessagesPerMinute: cfg.DeliveryDefaultRateLimitPerMinute,
		}
		backend := strings.ToLower(strings.TrimSpace(cfg.DeliveryRateLimitBackend))
		if backend == "redis" && redisClient != nil {
			handler.WithRateLimiter(delivery.NewRedisDomainRateLimiter(redisClient, "gogomail", rateLimitPolicy))
			logger.Info(
				"delivery rate limiting enabled",
				"backend", "redis",
				"default_per_minute", cfg.DeliveryDefaultRateLimitPerMinute,
				"domain_limits", cfg.DeliveryDomainRateLimitPerMinute,
			)
		} else {
			handler.WithRateLimiter(delivery.NewInMemoryDomainRateLimiter(rateLimitPolicy))
			logger.Info(
				"delivery rate limiting enabled",
				"backend", "memory",
				"default_per_minute", cfg.DeliveryDefaultRateLimitPerMinute,
				"domain_limits", cfg.DeliveryDomainRateLimitPerMinute,
			)
		}
	}
	if backoff := deliveryDomainBackoffFromConfig(cfg, redisClient); backoff != nil {
		handler.WithDomainBackoff(backoff)
		logger.Info(
			"delivery adaptive domain backoff enabled",
			"backend", cfg.DeliveryDomainBackoffBackend,
			"scope", cfg.DeliveryDomainBackoffScope,
			"base_delay", cfg.DeliveryDomainBackoffBaseDelay,
			"max_delay", cfg.DeliveryDomainBackoffMaxDelay,
		)
	}
	handler.WithMetrics(deliveryMetrics(cfg, logger))
	handler.WithLogger(logger)

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.DeliveryStream,
		Group:            cfg.DeliveryConsumerGroup,
		Consumer:         cfg.DeliveryConsumerName,
		Count:            int64(cfg.DeliveryConsumerCount),
		Block:            cfg.DeliveryConsumerBlock,
		ClaimIdle:        cfg.DeliveryConsumerClaimIdle,
		MaxDeliveries:    cfg.DeliveryConsumerMaxDeliveries,
		DeadLetterStream: cfg.DeliveryConsumerDeadLetterStream,
		Handler:          handler,
		Logger:           logger,
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
		"max_deliveries", cfg.DeliveryConsumerMaxDeliveries,
		"dead_letter_stream", cfg.DeliveryConsumerDeadLetterStream,
	)
	return consumer.Run(ctx)
}

