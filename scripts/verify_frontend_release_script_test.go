package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestVerifyFrontendReleaseRestoresNextGeneratedFiles(t *testing.T) {
	bodyBytes, err := os.ReadFile("verify-frontend-release.sh")
	if err != nil {
		t.Fatal(err)
	}
	body := string(bodyBytes)

	required := []string{
		"snapshot_next_generated_files",
		"restore_next_generated_files",
		"trap restore_next_generated_files EXIT",
		"apps/webmail/next-env.d.ts",
		"apps/console/next-env.d.ts",
		"apps/console/tsconfig.tsbuildinfo",
	}
	for _, needle := range required {
		if !strings.Contains(body, needle) {
			t.Fatalf("verify-frontend-release.sh must restore Next-generated file %q after build/type-check runs", needle)
		}
	}
}
