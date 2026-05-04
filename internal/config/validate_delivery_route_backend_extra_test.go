package config

import "testing"

func TestValidateRejectsUnknownDeliveryRouteBackend(t *testing.T) {
	t.Parallel()

	cfg := Load()
	cfg.DeliveryRouteBackend = "consul"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted unknown delivery route backend")
	}
}

func TestValidateRejectsPostgresDeliveryRoutesWithStaticSmartHost(t *testing.T) {
	t.Parallel()

	cfg := Load()
	cfg.DeliveryRouteBackend = "postgres"
	cfg.DeliverySmartHost = "relay.example.net"

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted postgres delivery routes combined with static smart host")
	}
}
