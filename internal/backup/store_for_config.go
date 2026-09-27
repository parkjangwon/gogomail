package backup

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/storage"
)

// StoreForConfig constructs the object store described by cfg. It mirrors the
// storage wiring used by the running application so that backup/restore observe
// the same backend the service uses. The returned store satisfies
// [storage.Store]; callers use it with [BuildStorageSnapshot].
func StoreForConfig(cfg config.Config) (storage.Store, error) {
	backend := normalizeBackend(cfg.StorageBackend)
	switch backend {
	case "local", "nfs":
		return storage.NewLocalStore(cfg.MailstoreRoot), nil
	case "s3", "minio":
		client, err := s3HTTPClient(cfg)
		if err != nil {
			return nil, err
		}
		return storage.NewS3Store(storage.S3Options{
			Endpoint:        cfg.StorageS3Endpoint,
			Region:          cfg.StorageS3Region,
			Bucket:          cfg.StorageS3Bucket,
			Prefix:          cfg.StorageS3Prefix,
			AccessKeyID:     cfg.StorageS3AccessKeyID,
			SecretAccessKey: cfg.StorageS3SecretAccessKey,
			SessionToken:    cfg.StorageS3SessionToken,
			ForcePathStyle:  cfg.StorageS3ForcePathStyle || backend == "minio",
			HTTPClient:      client,
			ReplicaRegion:   cfg.StorageS3ReplicaRegion,
			ReplicaBucket:   cfg.StorageS3ReplicaBucket,
		})
	default:
		return nil, fmt.Errorf("unsupported storage backend %q", cfg.StorageBackend)
	}
}

func s3HTTPClient(cfg config.Config) (*http.Client, error) {
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
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}, nil
}
