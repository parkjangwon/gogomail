package config

import (
	"testing"
	"time"
)

func TestLoadReadReplicaDefaults(t *testing.T) {
	t.Setenv("GOGOMAIL_DATABASE_REPLICA_URL", "postgres://gogomail:gogomail@replica:5432/gogomail?sslmode=require")
	t.Setenv("GOGOMAIL_DB_REPLICA_MAX_STALENESS", "15s")
	t.Setenv("GOGOMAIL_DB_REPLICA_FALLBACK_TO_PRIMARY", "false")

	cfg := Load()

	if cfg.DatabaseReplicaURL != "postgres://gogomail:gogomail@replica:5432/gogomail?sslmode=require" {
		t.Fatalf("DatabaseReplicaURL = %q", cfg.DatabaseReplicaURL)
	}
	if cfg.DBReplicaMaxStaleness != 15*time.Second {
		t.Fatalf("DBReplicaMaxStaleness = %v, want 15s", cfg.DBReplicaMaxStaleness)
	}
	if cfg.DBReplicaFallbackToPrimary {
		t.Fatal("DBReplicaFallbackToPrimary = true, want false")
	}
}

func TestValidateRejectsReplicaEqualToPrimary(t *testing.T) {
	cfg := Load()
	cfg.DatabaseURL = "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=require"
	cfg.DatabaseReplicaURL = cfg.DatabaseURL

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted replica URL identical to primary")
	}
}

func TestValidateRejectsNegativeReplicaStaleness(t *testing.T) {
	cfg := Load()
	cfg.DatabaseURL = "postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=require"
	cfg.DatabaseReplicaURL = "postgres://gogomail:gogomail@replica:5432/gogomail?sslmode=require"
	cfg.DBReplicaMaxStaleness = -1 * time.Second

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted negative replica staleness")
	}
}

func TestLoadS3ReplicaFields(t *testing.T) {
	t.Setenv("GOGOMAIL_STORAGE_S3_REPLICA_REGION", "us-west-2")
	t.Setenv("GOGOMAIL_STORAGE_S3_REPLICA_BUCKET", "gogomail-mail-dr")

	cfg := Load()

	if cfg.StorageS3ReplicaRegion != "us-west-2" {
		t.Fatalf("StorageS3ReplicaRegion = %q", cfg.StorageS3ReplicaRegion)
	}
	if cfg.StorageS3ReplicaBucket != "gogomail-mail-dr" {
		t.Fatalf("StorageS3ReplicaBucket = %q", cfg.StorageS3ReplicaBucket)
	}
}
