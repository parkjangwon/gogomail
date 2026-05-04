package maildb

import "testing"

func TestValidateCreateDeliveryRouteRequestAcceptsOperationalRoute(t *testing.T) {
	t.Parallel()

	err := ValidateCreateDeliveryRouteRequest(CreateDeliveryRouteRequest{
		DomainPattern: "*.example.net",
		Farm:          "transactional",
		Hosts:         []string{" Relay.EXAMPLE.net. ", "relay.example.net"},
		Port:          587,
		TLSMode:       "require",
		AuthUsername:  "relay-user",
		AuthPassword:  "secret",
	})
	if err != nil {
		t.Fatalf("ValidateCreateDeliveryRouteRequest returned error: %v", err)
	}
}

func TestNormalizeDeliveryRouteDomainPattern(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{
		" Example.COM ": "example.com",
		"*.Example.COM": "*.example.com",
		"*":             "*",
	} {
		got, err := normalizeDeliveryRouteDomainPattern(input)
		if err != nil {
			t.Fatalf("normalizeDeliveryRouteDomainPattern(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("normalizeDeliveryRouteDomainPattern(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeDeliveryRouteHostsDeduplicatesAndRejectsPorts(t *testing.T) {
	t.Parallel()

	hosts, err := normalizeDeliveryRouteHosts([]string{" Relay.EXAMPLE.net. ", "relay.example.net"})
	if err != nil {
		t.Fatalf("normalizeDeliveryRouteHosts returned error: %v", err)
	}
	if len(hosts) != 1 || hosts[0] != "relay.example.net" {
		t.Fatalf("hosts = %+v, want deduplicated relay.example.net", hosts)
	}

	if _, err := normalizeDeliveryRouteHosts([]string{"relay.example.net:587"}); err == nil {
		t.Fatal("normalizeDeliveryRouteHosts accepted host with port")
	}
}

func TestValidateCreateDeliveryRouteRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	for _, req := range []CreateDeliveryRouteRequest{
		{},
		{DomainPattern: "bad domain", Hosts: []string{"relay.example.net"}},
		{DomainPattern: "*.bad", Hosts: []string{"relay.example.net"}},
		{DomainPattern: "example.com"},
		{DomainPattern: "example.com", Hosts: []string{"relay.example.net"}, Port: 70000},
		{DomainPattern: "example.com", Hosts: []string{"relay.example.net"}, TLSMode: "cleartext"},
		{DomainPattern: "example.com", Hosts: []string{"relay.example.net"}, Description: "bad\nline"},
		{DomainPattern: "example.com", Hosts: []string{"relay.example.net"}, TLSMode: "disable", ImplicitTLS: true},
		{DomainPattern: "example.com", Hosts: []string{"relay.example.net"}, AuthPassword: "secret"},
	} {
		if err := ValidateCreateDeliveryRouteRequest(req); err == nil {
			t.Fatalf("ValidateCreateDeliveryRouteRequest(%+v) returned nil", req)
		}
	}
}

func TestValidateUpdateDeliveryRouteStatusRequest(t *testing.T) {
	t.Parallel()

	if err := ValidateUpdateDeliveryRouteStatusRequest(UpdateDeliveryRouteStatusRequest{ID: "route-1", Status: "disabled"}); err != nil {
		t.Fatalf("ValidateUpdateDeliveryRouteStatusRequest returned error: %v", err)
	}
	if err := ValidateUpdateDeliveryRouteStatusRequest(UpdateDeliveryRouteStatusRequest{ID: "route-1", Status: "paused"}); err == nil {
		t.Fatal("ValidateUpdateDeliveryRouteStatusRequest accepted unsupported status")
	}
}

func TestDeliveryRouteResolveViewCanRepresentDirectFallback(t *testing.T) {
	t.Parallel()

	view := DeliveryRouteResolveView{Domain: "example.net", Matched: false}
	if view.Route != nil {
		t.Fatalf("Route = %+v, want nil for direct fallback", view.Route)
	}
}
