package app

import (
	"context"
	"errors"
	"testing"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/maildb"
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

func TestPostgresDeliveryRouterMapsStoredRoute(t *testing.T) {
	t.Parallel()

	router := postgresDeliveryRouter{
		repository: fakeDeliveryRouteRepository{route: maildb.DeliveryRouteView{
			DomainPattern: "*.example.net",
			Farm:          "transactional",
			Hosts:         []string{"relay.example.net"},
			Port:          587,
			TLSMode:       "require",
			ImplicitTLS:   true,
			SMTPHello:     "mx.gogomail.example",
			PoolName:      "provider-a",
			AuthIdentity:  "tenant-a",
			AuthUsername:  "relay-user",
			AuthPassword:  "secret",
		}},
		fallbackTLSMode: delivery.DeliveryTLSRequire,
	}

	route, err := router.Route(context.Background(), delivery.Job{}, "mail.example.net")
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if route.Domain != "mail.example.net" || len(route.Hosts) != 1 || route.Hosts[0] != "relay.example.net" {
		t.Fatalf("route = %+v", route)
	}
	if route.Farm != "transactional" || route.Port != 587 || route.TLSMode != delivery.DeliveryTLSRequire || !route.ImplicitTLS {
		t.Fatalf("route = %+v", route)
	}
	if route.Auth.Username != "relay-user" || route.Auth.Password != "secret" || route.Auth.Identity != "tenant-a" {
		t.Fatalf("route auth = %+v", route.Auth)
	}
}

func TestPostgresDeliveryRouterFallsBackToDirectRouteWhenNoStoredRoute(t *testing.T) {
	t.Parallel()

	router := postgresDeliveryRouter{
		repository:      fakeDeliveryRouteRepository{err: maildb.ErrDeliveryRouteNotFound},
		fallbackTLSMode: delivery.DeliveryTLSRequire,
	}

	route, err := router.Route(context.Background(), delivery.Job{}, "example.net")
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(route.Hosts) != 0 {
		t.Fatalf("Hosts = %+v, want direct MX route", route.Hosts)
	}
	if route.TLSMode != delivery.DeliveryTLSRequire {
		t.Fatalf("TLSMode = %q, want fallback require", route.TLSMode)
	}
}

func TestPostgresDeliveryRouterReturnsLookupErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("database unavailable")
	router := postgresDeliveryRouter{repository: fakeDeliveryRouteRepository{err: want}}

	if _, err := router.Route(context.Background(), delivery.Job{}, "example.net"); !errors.Is(err, want) {
		t.Fatalf("Route error = %v, want %v", err, want)
	}
}

type fakeDeliveryRouteRepository struct {
	route maildb.DeliveryRouteView
	err   error
}

func (r fakeDeliveryRouteRepository) DeliveryRouteForDomain(context.Context, string) (maildb.DeliveryRouteView, error) {
	if r.err != nil {
		return maildb.DeliveryRouteView{}, r.err
	}
	return r.route, nil
}
