package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/pushnotify"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
)

func runAPIMeteringWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if strings.EqualFold(strings.TrimSpace(cfg.APIMeteringAggregateBackend), "disabled") {
		return waitForShutdown(ctx, logger, ModeAPIMeteringWorker)
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

	router := eventstream.NewRouter()
	if err := router.Register(apimeter.EventAPIUsage, apimeter.NewUsageHandler(apimeter.NewPostgresAggregateStore(db))); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.APIMeteringStream,
		Group:            cfg.APIMeteringConsumerGroup,
		Consumer:         cfg.APIMeteringConsumerName,
		Count:            int64(cfg.APIMeteringConsumerCount),
		Block:            cfg.APIMeteringConsumerBlock,
		ClaimIdle:        cfg.APIMeteringConsumerClaimIdle,
		MaxDeliveries:    cfg.APIMeteringConsumerMaxDeliveries,
		DeadLetterStream: cfg.APIMeteringConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"api metering worker started",
		"stream", cfg.APIMeteringStream,
		"group", cfg.APIMeteringConsumerGroup,
		"consumer", cfg.APIMeteringConsumerName,
		"backend", cfg.APIMeteringAggregateBackend,
		"max_deliveries", cfg.APIMeteringConsumerMaxDeliveries,
		"dead_letter_stream", cfg.APIMeteringConsumerDeadLetterStream,
	)
	return consumer.Run(ctx)
}

func runPushNotificationWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	backend := strings.ToLower(strings.TrimSpace(cfg.PushNotifyBackend))
	if backend == "" || backend == "none" {
		return waitForShutdown(ctx, logger, ModePushWorker)
	}

	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	repository := maildb.NewRepository(db)

	sink, err := pushNotificationSinkForConfig(cfg, logger, repository)
	if err != nil {
		return err
	}

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()
	pushRecorder := pushnotify.NewPostgresRecorder(db)
	router := eventstream.NewRouter()
	if err := router.Register(pushnotify.EventMailStored, pushnotify.NewHandler(
		sink,
		pushnotify.WithTargetResolver(pushnotify.NewDeviceResolver(repository, cfg.PushNotifyDeviceLimit)),
		pushnotify.WithCandidateRecorder(pushRecorder),
		pushnotify.WithOutcomeRecorder(pushRecorder),
	)); err != nil {
		return err
	}
	if err := router.Register(pushnotify.EventMailDeliveryExhausted,
		pushnotify.NewDeliveryExhaustedHandler(sink, repository),
	); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.PushNotifyConsumerGroup,
		Consumer:         cfg.PushNotifyConsumerName,
		Count:            int64(cfg.PushNotifyConsumerCount),
		Block:            cfg.PushNotifyConsumerBlock,
		ClaimIdle:        cfg.PushNotifyConsumerClaimIdle,
		MaxDeliveries:    cfg.PushNotifyConsumerMaxDeliveries,
		DeadLetterStream: cfg.PushNotifyConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"push notification worker started",
		"stream", cfg.EventStream,
		"group", cfg.PushNotifyConsumerGroup,
		"consumer", cfg.PushNotifyConsumerName,
		"backend", backend,
		"device_limit", cfg.PushNotifyDeviceLimit,
		"max_deliveries", cfg.PushNotifyConsumerMaxDeliveries,
		"dead_letter_stream", cfg.PushNotifyConsumerDeadLetterStream,
	)
	return consumer.Run(ctx)
}

func pushNotificationSinkForConfig(cfg config.Config, logger *slog.Logger, repo *maildb.Repository) (pushnotify.Sink, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.PushNotifyBackend)) {
	case "slog":
		return pushnotify.SlogSink{Logger: logger}, nil
	case "webhook":
		return pushnotify.NewWebhookSink(pushnotify.WebhookOptions{
			Endpoint: strings.TrimSpace(cfg.PushNotifyWebhookURL),
			Token:    cfg.PushNotifyWebhookToken,
			Client:   webhookguard.GuardedHTTPClient(&http.Client{Timeout: cfg.PushNotifyWebhookTimeout}, webhookguard.OutboundURLGuardOptions{}),
		})
	case "webpush":
		return pushnotify.NewWebPushSink(pushnotify.WebPushSinkOptions{
			VAPIDPublicKey:  cfg.WebPushVAPIDPublicKey,
			VAPIDPrivateKey: cfg.WebPushVAPIDPrivateKey,
			ContactEmail:    cfg.WebPushContactEmail,
			DB:              &webPushSubReaderAdapter{repo: repo},
			Logger:          logger,
		})
	default:
		return nil, errors.New("unsupported push notification backend")
	}
}

type webPushSubReaderAdapter struct {
	repo *maildb.Repository
}

func (a *webPushSubReaderAdapter) ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]pushnotify.WebPushSubData, error) {
	subs, err := a.repo.ListActiveWebPushSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]pushnotify.WebPushSubData, len(subs))
	for i, s := range subs {
		out[i] = pushnotify.WebPushSubData{
			ID:       s.ID,
			Endpoint: s.Endpoint,
			P256DH:   s.P256DH,
			Auth:     s.Auth,
		}
	}
	return out, nil
}

func (a *webPushSubReaderAdapter) SoftDeleteWebPushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	return a.repo.SoftDeleteWebPushSubscriptionByEndpoint(ctx, endpoint)
}

