package httpapi

import (
	"os"
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
