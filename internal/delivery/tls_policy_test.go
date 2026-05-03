package delivery

import (
	"crypto/tls"
	"testing"
)

func TestDeliveryTLSConfigDefaultsServerNameAndTLS12(t *testing.T) {
	t.Parallel()

	cfg := (&DirectSMTPTransport{}).deliveryTLSConfig("mx.example.net")
	if cfg.ServerName != "mx.example.net" {
		t.Fatalf("ServerName = %q, want mx.example.net", cfg.ServerName)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %#x, want TLS 1.2", cfg.MinVersion)
	}
}

func TestDeliveryTLSConfigRaisesWeakMinVersion(t *testing.T) {
	t.Parallel()

	transport := &DirectSMTPTransport{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS10}}
	cfg := transport.deliveryTLSConfig("mx.example.net")
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %#x, want TLS 1.2", cfg.MinVersion)
	}
}

func TestDeliveryTLSConfigClonesOperatorConfig(t *testing.T) {
	t.Parallel()

	base := &tls.Config{ServerName: "configured.example.net", MinVersion: tls.VersionTLS13}
	cfg := (&DirectSMTPTransport{TLSConfig: base}).deliveryTLSConfig("mx.example.net")
	if cfg == base {
		t.Fatal("deliveryTLSConfig returned original TLSConfig pointer")
	}
	if cfg.ServerName != "configured.example.net" {
		t.Fatalf("ServerName = %q, want configured name", cfg.ServerName)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %#x, want TLS 1.3", cfg.MinVersion)
	}
}
