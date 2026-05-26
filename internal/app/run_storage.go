package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/orgchart"
	"github.com/gogomail/gogomail/internal/storage"
)

func driveServiceForConfig(db *sql.DB, cfg config.Config, store storage.Store) *drive.Service {
	return drive.NewService(drive.NewRepository(db), storageStoresForConfig(cfg, store)).WithDefaultStorageBackend(normalizedStorageBackend(cfg.StorageBackend))
}

func orgChartServiceForDB(db *sql.DB) httpapi.OrgChartService {
	return orgchart.NewService(orgchart.NewRepository(db), nil)
}

func storageStoresForConfig(cfg config.Config, store storage.Store) map[string]storage.Store {
	backend := normalizedStorageBackend(cfg.StorageBackend)
	stores := map[string]storage.Store{
		backend: store,
	}
	if backend == "local" || backend == "nfs" {
		stores["local"] = store
		stores["nfs"] = store
	}
	if backend == "s3" || backend == "minio" {
		stores["s3"] = store
		stores["minio"] = store
	}
	for _, label := range cfg.StorageBackendCompatLabels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" {
			continue
		}
		stores[label] = store
	}
	return stores
}

func storageCapabilitiesForConfig(cfg config.Config) storage.BackendCapabilities {
	backend := normalizedStorageBackend(cfg.StorageBackend)
	labels := []string{backend}
	if backend == "local" || backend == "nfs" {
		labels = append(labels, "local", "nfs")
	}
	if backend == "s3" || backend == "minio" {
		labels = append(labels, "s3", "minio")
	}
	labels = append(labels, cfg.StorageBackendCompatLabels...)
	activeLabels := make([]string, 0, len(labels))
	seen := map[string]struct{}{}
	for _, label := range labels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		activeLabels = append(activeLabels, label)
	}
	sort.Strings(activeLabels)
	supportsLocalNFS, supportsMinIO, supportsAWSCompatible := storage.SupportMatrixForLabels(activeLabels)

	capabilities := storage.BackendCapabilities{
		ContractVersion:       httpapi.BackendContractVersion,
		ConfiguredBackend:     backend,
		BackendClass:          backend,
		ActiveLabels:          activeLabels,
		Operations:            []string{"put", "get", "get_range", "stat", "copy", "move", "list", "delete"},
		LocalFilesystem:       backend == "local" || backend == "nfs",
		S3Compatible:          backend == "s3" || backend == "minio",
		PathStyleAddressing:   false,
		CompatLabelsEnabled:   len(cfg.StorageBackendCompatLabels) > 0,
		ReadinessProbe:        true,
		SecretsRedacted:       true,
		SupportsBackendSwitch: true,
		SupportsLocalNFS:      supportsLocalNFS,
		SupportsMinIO:         supportsMinIO,
		SupportsAWSCompatible: supportsAWSCompatible,
		RequiresByteMigration: true,
	}
	if capabilities.S3Compatible {
		capabilities.BackendClass = "s3_compatible"
		capabilities.Region = strings.TrimSpace(cfg.StorageS3Region)
		capabilities.Bucket = strings.TrimSpace(cfg.StorageS3Bucket)
		capabilities.Prefix = strings.Trim(strings.TrimSpace(cfg.StorageS3Prefix), "/")
		endpointValue := strings.TrimSpace(cfg.StorageS3Endpoint)
		if endpointValue == "" && capabilities.Region != "" {
			endpointValue = "https://s3." + capabilities.Region + ".amazonaws.com"
		}
		if endpoint, err := storage.ValidateS3Endpoint(endpointValue); err == nil {
			capabilities.EndpointOrigin = endpoint.Scheme + "://" + endpoint.Host
			if endpoint.Path != "" && endpoint.Path != "/" {
				capabilities.EndpointOrigin += endpoint.EscapedPath()
			}
			capabilities.PathStyleAddressing = cfg.StorageS3ForcePathStyle || backend == "minio" || storage.S3BucketNeedsPathStyle(endpoint, capabilities.Bucket)
		} else {
			capabilities.PathStyleAddressing = cfg.StorageS3ForcePathStyle || backend == "minio"
		}
	} else if capabilities.LocalFilesystem {
		capabilities.BackendClass = "local"
	}
	return capabilities
}

func normalizedStorageBackend(value string) string {
	backend := strings.ToLower(strings.TrimSpace(value))
	if backend == "" {
		return "local"
	}
	return backend
}

type configuredObjectStore interface {
	storage.Store
	Check(context.Context) error
}

func objectStoreForConfig(cfg config.Config) (configuredObjectStore, error) {
	backend := normalizedStorageBackend(cfg.StorageBackend)
	switch backend {
	case "local", "nfs":
		return storage.NewLocalStore(cfg.MailstoreRoot), nil
	case "s3", "minio":
		opts, err := s3OptionsForConfig(cfg, backend)
		if err != nil {
			return nil, err
		}
		return storage.NewS3Store(opts)
	default:
		return nil, fmt.Errorf("unsupported storage backend %q", cfg.StorageBackend)
	}
}

func s3OptionsForConfig(cfg config.Config, backend string) (storage.S3Options, error) {
	backend = strings.ToLower(strings.TrimSpace(backend))
	client, err := s3HTTPClientForConfig(cfg)
	if err != nil {
		return storage.S3Options{}, err
	}
	return storage.S3Options{
		Endpoint:        cfg.StorageS3Endpoint,
		Region:          cfg.StorageS3Region,
		Bucket:          cfg.StorageS3Bucket,
		Prefix:          cfg.StorageS3Prefix,
		AccessKeyID:     cfg.StorageS3AccessKeyID,
		SecretAccessKey: cfg.StorageS3SecretAccessKey,
		SessionToken:    cfg.StorageS3SessionToken,
		ForcePathStyle:  cfg.StorageS3ForcePathStyle || backend == "minio",
		HTTPClient:      client,
	}, nil
}

func s3HTTPClientForConfig(cfg config.Config) (*http.Client, error) {
	caCertFile := strings.TrimSpace(cfg.StorageS3CACertFile)
	if caCertFile == "" && !cfg.StorageS3InsecureSkipVerify {
		return nil, nil
	}
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caCertFile != "" {
		data, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("read S3 CA certificate file: %w", err)
		}
		if !rootCAs.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("S3 CA certificate file must contain at least one PEM-encoded certificate")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            rootCAs,
		InsecureSkipVerify: cfg.StorageS3InsecureSkipVerify,
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}, nil
}

func storageReadinessCheck(name string, store interface {
	Check(context.Context) error
}) httpapi.ReadinessCheckFunc {
	return func(ctx context.Context) httpapi.ReadinessCheck {
		if store == nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: "storage is not configured"}
		}
		if err := store.Check(ctx); err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		return httpapi.ReadinessCheck{Name: name, Status: "ok", Detail: "probe ok"}
	}
}

func databaseReadinessCheck(name string, db *sql.DB, migrationDir string) httpapi.ReadinessCheckFunc {
	return func(ctx context.Context) httpapi.ReadinessCheck {
		if db == nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: "database handle is not configured"}
		}
		if err := db.PingContext(ctx); err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		current, expected, err := database.MigrationVersionReady(ctx, db, migrationDir)
		if err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		return httpapi.ReadinessCheck{
			Name:   name,
			Status: "ok",
			Detail: fmt.Sprintf("ping ok; migration version %d/%d", current, expected),
		}
	}
}

func redisReadinessCheck(name string, client *redis.Client) httpapi.ReadinessCheckFunc {
	return func(ctx context.Context) httpapi.ReadinessCheck {
		if client == nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: "redis client is not configured"}
		}
		if err := client.Ping(ctx).Err(); err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		return httpapi.ReadinessCheck{Name: name, Status: "ok", Detail: "ping ok"}
	}
}

// newRedisClient creates a Redis client. When RedisSentinelAddrs is non-empty a
// failover (Sentinel) client is returned; otherwise a plain single-node client.
func newRedisClient(cfg config.Config) *redis.Client {
	if len(cfg.RedisSentinelAddrs) > 0 {
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    cfg.RedisMasterName,
			SentinelAddrs: cfg.RedisSentinelAddrs,
			Password:      cfg.RedisPassword,
		})
	}
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
}

func openDatabase(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	return database.Open(ctx, cfg.DatabaseURL, database.Options{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
		ConnMaxIdleTime: cfg.DBConnMaxIdleTime,
	})
}

func tokenManagerForConfig(cfg config.Config, checker auth.RevocationChecker) (*auth.TokenManager, error) {
	if strings.TrimSpace(cfg.AuthJWTSecret) == "" {
		return nil, nil
	}
	tokenManager, err := auth.NewTokenManager(cfg.AuthJWTSecret)
	if err != nil {
		return nil, err
	}
	if checker != nil {
		tokenManager.SetRevocationChecker(checker)
	}
	return tokenManager, nil
}
