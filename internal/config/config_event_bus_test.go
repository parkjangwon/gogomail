package config

import (
	"testing"
)

func TestValidateAcceptsDefaultEventBusBackend(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	if cfg.EventBusBackend != "redis" {
		t.Fatalf("expected default event bus backend redis, got %q", cfg.EventBusBackend)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with default event bus backend: %v", err)
	}
}

func TestValidateRejectsUnknownEventBusBackend(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	cfg.EventBusBackend = "rabbitmq"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unknown event bus backend rejection")
	}
}

func TestValidateKafkaRequiresBrokers(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	cfg.EventBusBackend = "kafka"
	cfg.EventBusKafkaBrokers = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want kafka brokers required")
	}
}

func TestValidateKafkaAcceptsMinimalConfig(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	cfg.EventBusBackend = "kafka"
	cfg.EventBusKafkaBrokers = []string{"broker-1:9092", "broker-2:9092"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with minimal kafka config: %v", err)
	}
}

func TestValidateKafkaRejectsBadBrokerAddr(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	cfg.EventBusBackend = "kafka"
	cfg.EventBusKafkaBrokers = []string{"not-a-host-port"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want bad broker address rejection")
	}
}

func TestValidateKafkaRejectsBadRequiredAcks(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	cfg.EventBusBackend = "kafka"
	cfg.EventBusKafkaBrokers = []string{"broker-1:9092"}
	cfg.EventBusKafkaRequiredAcks = "some"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want bad required acks rejection")
	}
}

func TestValidateKafkaSASLRequiresCredentials(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	cfg.EventBusBackend = "kafka"
	cfg.EventBusKafkaBrokers = []string{"broker-1:9092"}
	cfg.EventBusKafkaSASLMechanism = "scram-sha-512"
	cfg.EventBusKafkaSASLUsername = ""
	cfg.EventBusKafkaSASLPassword = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want sasl credentials required")
	}
}

func TestValidateKafkaProductionRequiresTLSAndAcks(t *testing.T) {
	setDevelopmentMode(t)
	cfg := Load()
	setProductionSecrets(&cfg)
	cfg.Environment = "production"
	cfg.SubmissionAllowInsecureAuth = false
	cfg.IMAPAllowInsecureAuth = false
	cfg.CalDAVAllowInsecureAuth = false
	cfg.CardDAVAllowInsecureAuth = false
	cfg.EventBusBackend = "kafka"
	cfg.EventBusKafkaBrokers = []string{"broker-1:9092"}
	cfg.EventBusKafkaTLSEnabled = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want production kafka TLS requirement")
	}

	cfg.EventBusKafkaTLSEnabled = true
	cfg.EventBusKafkaRequiredAcks = "none"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want production kafka acks requirement")
	}

	cfg.EventBusKafkaRequiredAcks = "all"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with production-safe kafka config: %v", err)
	}
}
