package httpapi

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestOpenAPIDraftUsesBackendContractVersion(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}
	if !strings.Contains(string(raw), "version: "+BackendContractVersion) {
		t.Fatalf("OpenAPI draft does not contain backend contract version %q", BackendContractVersion)
	}
}

func TestOpenAPIDraftCoversRegisteredHTTPRoutes(t *testing.T) {
	t.Parallel()

	registered := extractRegisteredRoutes(t, "mail.go", "admin.go", "health.go")
	documented := extractOpenAPIRoutes(t, "../../docs/openapi.yaml")

	var missing []string
	for _, route := range registered {
		if !documented[route] {
			missing = append(missing, route)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("OpenAPI draft is missing registered routes:\n%s", strings.Join(missing, "\n"))
	}
}

func extractRegisteredRoutes(t *testing.T, filenames ...string) []string {
	t.Helper()

	pattern := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/(?:api|admin)/v1/[^"]+|/health/(?:live|ready))"`)
	var routes []string
	for _, filename := range filenames {
		raw, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read registered route source %s: %v", filename, err)
		}
		for _, match := range pattern.FindAllStringSubmatch(string(raw), -1) {
			routes = append(routes, match[1]+" "+normalizeOpenAPIPath(match[2]))
		}
	}
	sort.Strings(routes)
	return routes
}

func extractOpenAPIRoutes(t *testing.T, filename string) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read OpenAPI draft: %v", err)
	}

	routes := make(map[string]bool)
	var currentPath string
	pathPattern := regexp.MustCompile(`^  (/[^:]+):\s*$`)
	methodPattern := regexp.MustCompile(`^    (get|post|patch|delete):\s*$`)
	for _, line := range strings.Split(string(raw), "\n") {
		if match := pathPattern.FindStringSubmatch(line); match != nil {
			currentPath = match[1]
			continue
		}
		if currentPath == "" {
			continue
		}
		if match := methodPattern.FindStringSubmatch(line); match != nil {
			routes[strings.ToUpper(match[1])+" "+normalizeOpenAPIPath(currentPath)] = true
		}
	}
	return routes
}

func normalizeOpenAPIPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/api/v1")
	path = strings.TrimPrefix(path, "/admin/v1")
	return path
}
