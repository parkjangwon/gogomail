package config

import "testing"

func TestLoadDeliveryConcurrencyMaps(t *testing.T) {
	t.Setenv("GOGOMAIL_DELIVERY_FARM_CONCURRENCY", "general=50, bulk=10")
	t.Setenv("GOGOMAIL_DELIVERY_DOMAIN_CONCURRENCY", "Example.COM=5")

	cfg := Load()
	if cfg.DeliveryFarmConcurrency["general"] != 50 || cfg.DeliveryFarmConcurrency["bulk"] != 10 {
		t.Fatalf("farm concurrency = %+v", cfg.DeliveryFarmConcurrency)
	}
	if cfg.DeliveryDomainConcurrency["example.com"] != 5 {
		t.Fatalf("domain concurrency = %+v", cfg.DeliveryDomainConcurrency)
	}
}

func TestLoadDeliveryConcurrencyMapFallbackOnInvalid(t *testing.T) {
	t.Setenv("GOGOMAIL_DELIVERY_FARM_CONCURRENCY", "general:nope")

	cfg := Load()
	if cfg.DeliveryFarmConcurrency != nil {
		t.Fatalf("farm concurrency = %+v, want nil fallback", cfg.DeliveryFarmConcurrency)
	}
}
