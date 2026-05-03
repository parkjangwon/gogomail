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

func TestRoutePoolKeyIncludesFarmDomainHostAndTLSMode(t *testing.T) {
	t.Parallel()

	route := normalizeRoute(Job{QueuedMessage: QueuedMessage{Farm: outbound.FarmBulk}}, "Example.NET", Route{
		Domain:  "Example.NET",
		TLSMode: DeliveryTLSRequire,
	})
	got := routePoolKey(route, "MX-A.Example.NET")
	want := "bulk|example.net|mx-a.example.net:25|require"
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
	want := "provider-a|example.net|mx.example.net:2525|disable"
	if got != want {
		t.Fatalf("routePoolKey = %q, want %q", got, want)
	}
}
