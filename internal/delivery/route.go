package delivery

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogomail/gogomail/internal/outbound"
)

type Route struct {
	Farm     outbound.Farm
	Domain   string
	Hosts    []string
	Port     int
	Hello    string
	TLSMode  DeliveryTLSMode
	PoolName string
}

type Router interface {
	Route(ctx context.Context, job Job, domain string) (Route, error)
}

func normalizeRoute(job Job, domain string, route Route) Route {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if route.Farm == "" {
		route.Farm = job.Farm
	}
	if strings.TrimSpace(route.Domain) == "" {
		route.Domain = domain
	} else {
		route.Domain = strings.ToLower(strings.TrimSpace(route.Domain))
	}
	route.Hosts = normalizeRouteHosts(route.Hosts)
	if route.Port <= 0 {
		route.Port = 25
	}
	route.Hello = strings.TrimSpace(route.Hello)
	route.TLSMode = normalizeDeliveryTLSMode(route.TLSMode)
	route.PoolName = strings.TrimSpace(route.PoolName)
	return route
}

func normalizeRouteHosts(hosts []string) []string {
	out := hosts[:0]
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" || host == "." {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

func routePoolKey(route Route, host string) string {
	pool := strings.TrimSpace(route.PoolName)
	if pool == "" {
		pool = string(route.Farm)
	}
	domain := strings.ToLower(strings.TrimSpace(route.Domain))
	host = strings.ToLower(strings.TrimSpace(host))
	return pool + "|" + domain + "|" + host + ":" + strconv.Itoa(normalizeRoutePort(route.Port)) + "|" + string(normalizeDeliveryTLSMode(route.TLSMode))
}

func normalizeRoutePort(port int) int {
	if port <= 0 {
		return 25
	}
	return port
}
