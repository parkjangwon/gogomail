package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailflow"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/searchindex"
)

func runSearchIndexWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "disabled") {
		return waitForShutdown(ctx, logger, ModeSearchIndexWorker)
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

	repository := maildb.NewRepository(db)
	indexer, err := searchIndexerForConfig(cfg, repository)
	if err != nil {
		return err
	}
	if err := maybeBootstrapSearchIndex(ctx, cfg, indexer); err != nil {
		return err
	}
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	router := eventstream.NewRouter()
	if err := router.Register("mail.stored", searchindex.NewHandler(
		searchindex.NewStorageStoreReader(store),
		indexer,
		searchindex.HandlerOptions{MaxTextBodyBytes: cfg.SearchIndexMaxBodyBytes},
	)); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.SearchIndexConsumerGroup,
		Consumer:         cfg.SearchIndexConsumerName,
		Count:            int64(cfg.SearchIndexConsumerCount),
		Block:            cfg.SearchIndexConsumerBlock,
		ClaimIdle:        cfg.SearchIndexConsumerClaimIdle,
		MaxDeliveries:    cfg.SearchIndexConsumerMaxDeliveries,
		DeadLetterStream: cfg.SearchIndexConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
	})
	if err != nil {
		return err
	}

	logger.Info("search index worker started", searchIndexWorkerLogFields(cfg)...)
	return consumer.Run(ctx)
}

func searchIndexWorkerLogFields(cfg config.Config) []any {
	fields := []any{
		"stream", cfg.EventStream,
		"group", cfg.SearchIndexConsumerGroup,
		"consumer", cfg.SearchIndexConsumerName,
		"backend", strings.ToLower(strings.TrimSpace(cfg.SearchIndexBackend)),
		"max_body_bytes", cfg.SearchIndexMaxBodyBytes,
		"max_deliveries", cfg.SearchIndexConsumerMaxDeliveries,
		"dead_letter_stream", cfg.SearchIndexConsumerDeadLetterStream,
	}
	if strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		fields = append(fields,
			"opensearch_index", strings.TrimSpace(cfg.SearchIndexOpenSearchIndex),
			"opensearch_bootstrap", cfg.SearchIndexOpenSearchBootstrap,
		)
	}
	return fields
}

func searchIndexerForConfig(cfg config.Config, repository *maildb.Repository) (searchindex.Indexer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.SearchIndexBackend)) {
	case "postgres":
		return searchindex.NewPostgresIndexer(repository), nil
	case "opensearch":
		return searchindex.NewOpenSearchIndexer(openSearchOptionsForConfig(cfg))
	default:
		return nil, fmt.Errorf("unsupported search index backend %q", cfg.SearchIndexBackend)
	}
}

type searchIndexBootstrapper interface {
	EnsureIndex(ctx context.Context) error
}

func maybeBootstrapSearchIndex(ctx context.Context, cfg config.Config, indexer any) error {
	if !cfg.SearchIndexOpenSearchBootstrap {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		return nil
	}
	bootstrapper, ok := indexer.(searchIndexBootstrapper)
	if !ok {
		return fmt.Errorf("search index backend %q does not support bootstrap", cfg.SearchIndexBackend)
	}
	return bootstrapper.EnsureIndex(ctx)
}

func searchIDSourceForConfig(cfg config.Config) (mailservice.SearchIDSource, error) {
	if !strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		return nil, nil
	}
	return searchindex.NewOpenSearchSearcher(openSearchOptionsForConfig(cfg))
}

func openSearchOptionsForConfig(cfg config.Config) searchindex.OpenSearchOptions {
	timeout := cfg.SearchIndexOpenSearchTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return searchindex.OpenSearchOptions{
		Endpoint:       cfg.SearchIndexOpenSearchEndpoint,
		Index:          cfg.SearchIndexOpenSearchIndex,
		Client:         &http.Client{Timeout: timeout},
		Username:       cfg.SearchIndexOpenSearchUsername,
		Password:       cfg.SearchIndexOpenSearchPassword,
		KoreanAnalyzer: cfg.SearchIndexOpenSearchKoreanAnalyzer,
	}
}

func mailFlowOpenSearchOptionsForConfig(cfg config.Config) searchindex.OpenSearchOptions {
	timeout := cfg.SearchIndexOpenSearchTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return searchindex.OpenSearchOptions{
		Endpoint: cfg.SearchIndexOpenSearchEndpoint,
		Index:    cfg.MailFlowOpenSearchIndex,
		Client:   &http.Client{Timeout: timeout},
		Username: cfg.SearchIndexOpenSearchUsername,
		Password: cfg.SearchIndexOpenSearchPassword,
	}
}

func mailFlowStatsProviderForConfig(cfg config.Config, repo *maildb.Repository) mailflow.MailFlowStatsProvider {
	backend := strings.ToLower(strings.TrimSpace(cfg.MailFlowStatsBackend))
	if backend == "" {
		backend = "auto"
	}
	switch backend {
	case "postgres":
		return mailflow.NewPostgresMailFlowStatsProvider(repo)
	case "opensearch":
		searcher, err := searchindex.NewMailFlowStatsSearcher(mailFlowOpenSearchOptionsForConfig(cfg))
		if err != nil {
			logger := slog.Default()
			logger.Warn("failed to create mail flow OpenSearch stats searcher, falling back to postgres", "error", err)
			return mailflow.NewPostgresMailFlowStatsProvider(repo)
		}
		return mailflow.NewOpenSearchMailFlowStatsProvider(&searcher)
	case "auto":
		if !cfg.MailFlowOpenSearchBootstrap {
			return mailflow.NewPostgresMailFlowStatsProvider(repo)
		}
		searcher, err := searchindex.NewMailFlowStatsSearcher(mailFlowOpenSearchOptionsForConfig(cfg))
		if err != nil {
			logger := slog.Default()
			logger.Warn("failed to create mail flow OpenSearch stats searcher, falling back to postgres", "error", err)
			return mailflow.NewPostgresMailFlowStatsProvider(repo)
		}
		return mailflow.NewOpenSearchMailFlowStatsProvider(&searcher)
	default:
		logger := slog.Default()
		logger.Warn("unknown mail flow stats backend, using postgres", "backend", backend)
		return mailflow.NewPostgresMailFlowStatsProvider(repo)
	}
}
