package config

import "testing"

func TestLoadInboundTrustedRelays(t *testing.T) {
	t.Setenv("GOGOMAIL_INBOUND_TRUSTED_RELAYS", "10.0.0.0/8, 192.0.2.1")

	cfg := Load()
	if len(cfg.InboundTrustedRelays) != 2 {
		t.Fatalf("InboundTrustedRelays = %+v", cfg.InboundTrustedRelays)
	}
	if cfg.InboundTrustedRelays[0] != "10.0.0.0/8" || cfg.InboundTrustedRelays[1] != "192.0.2.1" {
		t.Fatalf("InboundTrustedRelays = %+v", cfg.InboundTrustedRelays)
	}
}
