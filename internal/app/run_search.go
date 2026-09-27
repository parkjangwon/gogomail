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
	maybePingSearchBackend(ctx, cfg, indexer, logger)
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

// searchBackendPinger is implemented by search backends that can report
// cluster reachability (currently OpenSearch).
type searchBackendPinger interface {
	Ping(ctx context.Context) error
}

// mappingVersionReporter is implemented by search backends that stamp and can
// read back a mapping version (currently OpenSearch).
type mappingVersionReporter interface {
	MappingVersion(ctx context.Context) (string, error)
}

// maybePingSearchBackend performs a best-effort health check against the
// search backend at worker startup. A failure is logged as a warning rather
// than fatal: the cluster may still be starting, and the consumer loop retries
// indexing via the at-least-once event stream. When the backend exposes a
// mapping version, a mismatch against the current mapping is surfaced so
// operators know a rollover/reindex is due.
func maybePingSearchBackend(ctx context.Context, cfg config.Config, indexer any, logger *slog.Logger) {
	if !strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		return
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if pinger, ok := indexer.(searchBackendPinger); ok {
		if err := pinger.Ping(pingCtx); err != nil {
			logger.Warn("opensearch health check failed at startup; worker will retry via event stream", "error", err)
			return
		}
		logger.Info("opensearch health check passed")
	}
	if reporter, ok := indexer.(mappingVersionReporter); ok {
		version, err := reporter.MappingVersion(pingCtx)
		if err != nil {
			logger.Warn("opensearch mapping version check failed", "error", err)
			return
		}
		if version != "" && version != searchindex.OpenSearchMappingVersion {
			logger.Warn("opensearch index mapping version is stale; a rollover/reindex is recommended",
				"index_mapping_version", version,
				"expected_mapping_version", searchindex.OpenSearchMappingVersion,
			)
		}
	}
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
