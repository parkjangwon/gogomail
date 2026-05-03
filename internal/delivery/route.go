package delivery

import (
	"context"
	"net"
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
	Auth     RouteAuth
}

type RouteAuth struct {
	Identity string
	Username string
	Password string
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
	var hostPort int
	route.Hosts, hostPort = normalizeRouteHostsAndPort(route.Hosts)
	if route.Port <= 0 || route.Port > 65535 {
		route.Port = hostPort
	}
	if route.Port <= 0 || route.Port > 65535 {
		route.Port = 25
	}
	route.Hello = strings.TrimSpace(route.Hello)
	route.TLSMode = normalizeDeliveryTLSMode(route.TLSMode)
	route.PoolName = strings.TrimSpace(route.PoolName)
	route.Auth = normalizeRouteAuth(route.Auth)
	return route
}

func normalizeRouteHosts(hosts []string) []string {
	normalized, _ := normalizeRouteHostsAndPort(hosts)
	return normalized
}

func normalizeRouteHostsAndPort(hosts []string) ([]string, int) {
	out := hosts[:0]
	seen := make(map[string]struct{}, len(hosts))
	var detectedPort int
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" || host == "." {
			continue
		}
		if parsedHost, parsedPort, ok := splitRouteHostPort(host); ok {
			host = parsedHost
			if detectedPort <= 0 {
				detectedPort = parsedPort
			}
		} else {
			host = strings.Trim(host, "[]")
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out, detectedPort
}

func splitRouteHostPort(value string) (string, int, bool) {
	host, portValue, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, false
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, false
	}
	host = strings.ToLower(strings.Trim(strings.TrimSuffix(strings.TrimSpace(host), "."), "[]"))
	if host == "" || host == "." {
		return "", 0, false
	}
	return host, port, true
}

func routePoolKey(route Route, host string) string {
	pool := strings.TrimSpace(route.PoolName)
	if pool == "" {
		pool = string(route.Farm)
	}
	domain := strings.ToLower(strings.TrimSpace(route.Domain))
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	authUser := strings.ToLower(strings.TrimSpace(route.Auth.Username))
	authIdentity := strings.ToLower(strings.TrimSpace(route.Auth.Identity))
	return pool + "|" + domain + "|" + host + ":" + strconv.Itoa(normalizeRoutePort(route.Port)) + "|" + string(normalizeDeliveryTLSMode(route.TLSMode)) + "|auth=" + authUser + "|identity=" + authIdentity
}

func normalizeRoutePort(port int) int {
	if port <= 0 || port > 65535 {
		return 25
	}
	return port
}

func normalizeRouteAuth(auth RouteAuth) RouteAuth {
	auth.Identity = strings.TrimSpace(auth.Identity)
	auth.Username = strings.TrimSpace(auth.Username)
	auth.Password = strings.TrimSpace(auth.Password)
	return auth
}

func routeRequiresAuth(route Route) bool {
	return strings.TrimSpace(route.Auth.Username) != ""
}
