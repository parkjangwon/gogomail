package config

import "testing"

func TestLoadReadsDeliverySmartHostEnvironment(t *testing.T) {
	t.Setenv("GOGOMAIL_DELIVERY_SMARTHOST", " smtp.relay.example.net:587 ")
	t.Setenv("GOGOMAIL_DELIVERY_SMARTHOST_PORT", "2525")
	t.Setenv("GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE", "require")
	t.Setenv("GOGOMAIL_DELIVERY_SMARTHOST_USERNAME", " relay-user ")
	t.Setenv("GOGOMAIL_DELIVERY_SMARTHOST_PASSWORD", " secret ")
	t.Setenv("GOGOMAIL_DELIVERY_SMARTHOST_IDENTITY", " tenant-a ")

	cfg := Load()

	if cfg.DeliverySmartHost != " smtp.relay.example.net:587 " {
		t.Fatalf("DeliverySmartHost = %q, want raw configured host", cfg.DeliverySmartHost)
	}
	if cfg.DeliverySmartHostPort != 2525 {
		t.Fatalf("DeliverySmartHostPort = %d, want 2525", cfg.DeliverySmartHostPort)
	}
	if cfg.DeliverySmartHostTLSMode != "require" {
		t.Fatalf("DeliverySmartHostTLSMode = %q, want require", cfg.DeliverySmartHostTLSMode)
	}
	if cfg.DeliverySmartHostUsername != " relay-user " ||
		cfg.DeliverySmartHostPassword != " secret " ||
		cfg.DeliverySmartHostIdentity != " tenant-a " {
		t.Fatalf("smart-host credentials not loaded: user=%q identity=%q", cfg.DeliverySmartHostUsername, cfg.DeliverySmartHostIdentity)
	}
}
