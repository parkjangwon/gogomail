package backup

import (
	"github.com/gogomail/gogomail/internal/config"
)

// ConfigSnapshot is a redacted, restore-relevant view of the running
// configuration captured in a backup bundle. It intentionally excludes secret
// material: every field that could carry a credential is either omitted or
// replaced with [RedactedPlaceholder]. The snapshot exists to document the
// deployment shape that produced the backup (which storage backend, which
// migration directory, whether replicas are configured) so an operator can
// reconstruct a compatible target — not to reconstruct secrets.
type ConfigSnapshot struct {
	Environment    string `json:"environment"`
	StorageBackend string `json:"storage_backend"`
	MigrationDir   string `json:"migration_dir"`
	MailstoreRoot  string `json:"mailstore_root,omitempty"`

	// S3 topology — non-secret coordinates only. Access keys and session tokens
	// are never captured.
	StorageS3Endpoint      string `json:"storage_s3_endpoint,omitempty"`
	StorageS3Region        string `json:"storage_s3_region,omitempty"`
	StorageS3Bucket        string `json:"storage_s3_bucket,omitempty"`
	StorageS3Prefix        string `json:"storage_s3_prefix,omitempty"`
	StorageS3ReplicaRegion string `json:"storage_s3_replica_region,omitempty"`
	StorageS3ReplicaBucket string `json:"storage_s3_replica_bucket,omitempty"`

	// DatabaseConfigured / ReplicaConfigured record whether these DSNs were set
	// without leaking the DSN (which may embed a password). RedactedSecrets lists
	// the configuration keys whose values were present but deliberately withheld.
	DatabaseConfigured bool     `json:"database_configured"`
	ReplicaConfigured  bool     `json:"replica_configured"`
	RedactedSecrets    []string `json:"redacted_secrets"`
}

// secretConfigKeys enumerates every configuration field that may hold sensitive
// material. When such a field is non-empty in the source config its key is
// recorded in ConfigSnapshot.RedactedSecrets, but its value is never written to
// the bundle.
var secretConfigKeys = []struct {
	key     string
	present func(config.Config) bool
}{
	{"GOGOMAIL_DATABASE_URL", func(c config.Config) bool { return c.DatabaseURL != "" }},
	{"GOGOMAIL_DATABASE_REPLICA_URL", func(c config.Config) bool { return c.DatabaseReplicaURL != "" }},
	{"GOGOMAIL_REDIS_PASSWORD", func(c config.Config) bool { return c.RedisPassword != "" }},
	{"GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID", func(c config.Config) bool { return c.StorageS3AccessKeyID != "" }},
	{"GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY", func(c config.Config) bool { return c.StorageS3SecretAccessKey != "" }},
	{"GOGOMAIL_STORAGE_S3_SESSION_TOKEN", func(c config.Config) bool { return c.StorageS3SessionToken != "" }},
	{"GOGOMAIL_SCIM_TOKEN", func(c config.Config) bool { return c.SCIMToken != "" }},
	{"GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN", func(c config.Config) bool { return c.AttachmentScanWebhookToken != "" }},
}

// NewConfigSnapshot builds a redacted [ConfigSnapshot] from a live config.
// It records the presence of each secret field in RedactedSecrets but never
// copies a secret value into the returned snapshot.
func NewConfigSnapshot(cfg config.Config) ConfigSnapshot {
	snap := ConfigSnapshot{
		Environment:            cfg.Environment,
		StorageBackend:         normalizeBackend(cfg.StorageBackend),
		MigrationDir:           cfg.MigrationDir,
		MailstoreRoot:          cfg.MailstoreRoot,
		StorageS3Endpoint:      cfg.StorageS3Endpoint,
		StorageS3Region:        cfg.StorageS3Region,
		StorageS3Bucket:        cfg.StorageS3Bucket,
		StorageS3Prefix:        cfg.StorageS3Prefix,
		StorageS3ReplicaRegion: cfg.StorageS3ReplicaRegion,
		StorageS3ReplicaBucket: cfg.StorageS3ReplicaBucket,
		DatabaseConfigured:     cfg.DatabaseURL != "",
		ReplicaConfigured:      cfg.DatabaseReplicaURL != "",
	}
	for _, sk := range secretConfigKeys {
		if sk.present(cfg) {
			snap.RedactedSecrets = append(snap.RedactedSecrets, sk.key)
		}
	}
	return snap
}
