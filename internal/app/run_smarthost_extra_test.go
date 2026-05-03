package app

import (
	"context"
	"testing"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/delivery"
)

func TestDeliveryRouterFromConfigBuildsSmartHostRoute(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		DeliverySmartHost:            " SMTP.RELAY.EXAMPLE.NET:587 ",
		DeliverySmartHostTLSMode:     "require",
		DeliverySmartHostImplicitTLS: true,
		DeliverySmartHostUsername:    " relay-user ",
		DeliverySmartHostPassword:    " secret ",
		DeliverySmartHostIdentity:    " tenant-a ",
	}

	router := deliveryRouterFromConfig(cfg)
	if router == nil {
		t.Fatal("deliveryRouterFromConfig returned nil")
	}
	route, err := router.Route(context.Background(), delivery.Job{}, "Example.NET")
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Domain != "Example.NET" {
		t.Fatalf("Domain = %q, want requested domain before transport normalization", route.Domain)
	}
	if len(route.Hosts) != 1 || route.Hosts[0] != " SMTP.RELAY.EXAMPLE.NET:587 " {
		t.Fatalf("route hosts = %+v, want configured smart-host", route.Hosts)
	}
	if route.TLSMode != delivery.DeliveryTLSRequire {
		t.Fatalf("TLSMode = %q, want require", route.TLSMode)
	}
	if !route.ImplicitTLS {
		t.Fatal("ImplicitTLS = false, want true")
	}
	if route.Auth.Username != " relay-user " || route.Auth.Identity != " tenant-a " || route.Auth.Password != " secret " {
		t.Fatalf("Auth = %+v, want configured smart-host credentials before transport normalization", route.Auth)
	}
}

func TestDeliveryRouterFromConfigSkipsBlankSmartHost(t *testing.T) {
	t.Parallel()

	if router := deliveryRouterFromConfig(config.Config{}); router != nil {
		t.Fatalf("deliveryRouterFromConfig returned %#v, want nil", router)
	}
}
