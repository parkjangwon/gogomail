package backup

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/config"
)

func TestNewConfigSnapshotRedactsSecretsAndNeverEmitsValues(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Environment:              "production",
		StorageBackend:           "s3",
		MigrationDir:             "migrations",
		DatabaseURL:              "postgres://user:SUPERSECRETPW@db:5432/gogomail",
		DatabaseReplicaURL:       "postgres://user:REPLICAPW@replica:5432/gogomail",
		RedisPassword:            "REDISPW",
		StorageS3Bucket:          "gogomail-prod",
		StorageS3Region:          "us-east-1",
		StorageS3AccessKeyID:     "AKIAEXAMPLE",
		StorageS3SecretAccessKey: "SECRETKEYVALUE",
		StorageS3SessionToken:    "SESSIONTOKENVALUE",
		SCIMToken:                "SCIMTOKENVALUE",
	}

	snap := NewConfigSnapshot(cfg)

	// Non-secret coordinates are preserved.
	if snap.StorageBackend != "s3" || snap.StorageS3Bucket != "gogomail-prod" || snap.StorageS3Region != "us-east-1" {
		t.Fatalf("snapshot dropped non-secret fields: %+v", snap)
	}
	if !snap.DatabaseConfigured || !snap.ReplicaConfigured {
		t.Fatalf("expected database/replica configured flags: %+v", snap)
	}

	// Every secret key must be reported as redacted.
	wantKeys := []string{
		"GOGOMAIL_DATABASE_URL",
		"GOGOMAIL_DATABASE_REPLICA_URL",
		"GOGOMAIL_REDIS_PASSWORD",
		"GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID",
		"GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY",
		"GOGOMAIL_STORAGE_S3_SESSION_TOKEN",
		"GOGOMAIL_SCIM_TOKEN",
	}
	for _, k := range wantKeys {
		if !slices.Contains(snap.RedactedSecrets, k) {
			t.Fatalf("RedactedSecrets missing %q: %v", k, snap.RedactedSecrets)
		}
	}

	// The serialized snapshot must not contain any raw secret value.
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	serialized := string(data)
	for _, secret := range []string{
		"SUPERSECRETPW", "REPLICAPW", "REDISPW",
		"AKIAEXAMPLE", "SECRETKEYVALUE", "SESSIONTOKENVALUE", "SCIMTOKENVALUE",
	} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("serialized snapshot leaked secret %q: %s", secret, serialized)
		}
	}
}

func TestNewConfigSnapshotOmitsAbsentSecrets(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Environment:    "development",
		StorageBackend: "local",
		MailstoreRoot:  "var/mailstore",
		MigrationDir:   "migrations",
	}
	snap := NewConfigSnapshot(cfg)
	if len(snap.RedactedSecrets) != 0 {
		t.Fatalf("RedactedSecrets = %v, want empty for a secret-free config", snap.RedactedSecrets)
	}
	if snap.DatabaseConfigured {
		t.Fatal("DatabaseConfigured should be false when DatabaseURL is empty")
	}
	if snap.StorageBackend != "local" {
		t.Fatalf("StorageBackend = %q, want local", snap.StorageBackend)
	}
}
