package config

import "testing"

func TestLoadReadsDeliveryRouteBackend(t *testing.T) {
	t.Setenv("GOGOMAIL_DELIVERY_ROUTE_BACKEND", "postgres")

	cfg := Load()
	if cfg.DeliveryRouteBackend != "postgres" {
		t.Fatalf("DeliveryRouteBackend = %q, want postgres", cfg.DeliveryRouteBackend)
	}
}
