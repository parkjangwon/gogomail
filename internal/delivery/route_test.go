package delivery

import (
	"testing"

	"github.com/gogomail/gogomail/internal/outbound"
)

func TestNormalizeRouteCleansHostsAndDefaults(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmBulk}}, "Example.NET", Route{
		Hosts: []string{" MX-A.Example.NET. ", "mx-a.example.net", ".", "", "MX-B.Example.NET."},
	})
	if route.Farm != outbound.FarmBulk {
		t.Fatalf("Farm = %q, want bulk", route.Farm)
	}
	if route.Domain != "example.net" {
		t.Fatalf("Domain = %q, want example.net", route.Domain)
	}
	if route.Port != 25 {
		t.Fatalf("Port = %d, want default SMTP port 25", route.Port)
	}
	want := []string{"mx-a.example.net", "mx-b.example.net"}
	if len(route.Hosts) != len(want) {
		t.Fatalf("Hosts = %+v, want %+v", route.Hosts, want)
	}
	for i := range want {
		if route.Hosts[i] != want[i] {
			t.Fatalf("Hosts = %+v, want %+v", route.Hosts, want)
		}
	}
}

func TestNormalizeRouteAcceptsHostPortSmartHost(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral}}, "Example.NET", Route{
		Hosts: []string{" SMTP.EXAMPLE.NET:587 ", "smtp.example.net"},
	})
	if route.Port != 587 {
		t.Fatalf("Port = %d, want smart-host port 587", route.Port)
	}
	if len(route.Hosts) != 1 || route.Hosts[0] != "smtp.example.net" {
		t.Fatalf("Hosts = %+v, want host without port", route.Hosts)
	}
}

func TestNormalizeRouteExplicitPortOverridesHostPort(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral}}, "Example.NET", Route{
		Hosts: []string{"smtp.example.net:587"},
		Port:  2525,
	})
	if route.Port != 2525 {
		t.Fatalf("Port = %d, want explicit route port", route.Port)
	}
	if len(route.Hosts) != 1 || route.Hosts[0] != "smtp.example.net" {
		t.Fatalf("Hosts = %+v, want host without port", route.Hosts)
	}
}

func TestRoutePoolKeyIncludesFarmDomainHostAndTLSMode(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmBulk}}, "Example.NET", Route{
		Domain:  "Example.NET",
		TLSMode: DeliveryTLSRequire,
	})
	got := routePoolKey(route, "MX-A.Example.NET")
	want := "bulk|example.net|mx-a.example.net:25|require|auth="
	if got != want {
		t.Fatalf("routePoolKey = %q, want %q", got, want)
	}
}

func TestRoutePoolKeyUsesExplicitPoolName(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmBulk}}, "example.net", Route{
		PoolName: "provider-a",
		Port:     2525,
		TLSMode:  DeliveryTLSDisable,
	})
	got := routePoolKey(route, "mx.example.net")
	want := "provider-a|example.net|mx.example.net:2525|disable|auth="
	if got != want {
		t.Fatalf("routePoolKey = %q, want %q", got, want)
	}
}

func TestRoutePoolKeySeparatesAuthenticatedRoutes(t *testing.T) {
	t.Parallel()

	base := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral}}, "example.net", Route{
		Auth: RouteAuth{Username: "relay-a"},
	})
	other := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmGeneral}}, "example.net", Route{
		Auth: RouteAuth{Username: "relay-b"},
	})
	if routePoolKey(base, "mx.example.net") == routePoolKey(other, "mx.example.net") {
		t.Fatal("routePoolKey collapsed distinct authenticated routes")
	}
}

func TestNormalizeRouteAuthTrimsCredentials(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{}, "example.net", Route{
		Auth: RouteAuth{
			Identity: " identity ",
			Username: " relay-user ",
			Password: " secret ",
		},
	})
	if route.Auth.Identity != "identity" || route.Auth.Username != "relay-user" || route.Auth.Password != "secret" {
		t.Fatalf("route auth = %+v", route.Auth)
	}
	if !routeRequiresAuth(route) {
		t.Fatal("routeRequiresAuth = false, want true")
	}
}

func TestRouteDoesNotRequireAuthWithoutUsername(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{}, "example.net", Route{Auth: RouteAuth{Password: "secret"}})
	if routeRequiresAuth(route) {
		t.Fatal("routeRequiresAuth = true without username")
	}
}
