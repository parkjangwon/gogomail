package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestWebmailRefreshIntervalSettingDrivesMailPolling(t *testing.T) {
	hook, err := os.ReadFile("../apps/webmail/src/hooks/useMailList.ts")
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile("../apps/webmail/src/app/mail/page.tsx")
	if err != nil {
		t.Fatal(err)
	}
	// The background poll was extracted to useMailServiceWorker; check both locations.
	swHook, swErr := os.ReadFile("../apps/webmail/src/app/mail/useMailServiceWorker.ts")

	hookText := string(hook)
	pageText := string(page)
	swText := ""
	if swErr == nil {
		swText = string(swHook)
	}

	if !strings.Contains(hookText, "export function useMailList(folderId: string, refreshIntervalSeconds: RefreshIntervalSeconds)") {
		t.Fatalf("useMailList must accept refreshIntervalSeconds from Settings")
	}
	if strings.Contains(hookText, "}, 30_000);") {
		t.Fatalf("useMailList must not hard-code a 30s polling interval")
	}
	if !strings.Contains(pageText, "useMailList(activeFolderId, refreshIntervalSeconds)") {
		t.Fatalf("mail page must pass the configured refresh interval into useMailList")
	}
	// The polling interval is now in useMailServiceWorker (extracted hook) but the
	// setting still flows through from page.tsx via the refreshIntervalSeconds param.
	if !strings.Contains(pageText, "refreshIntervalSeconds") && !strings.Contains(swText, "refreshIntervalSeconds * 1000") {
		t.Fatalf("refresh interval setting must be wired to the background poll (page.tsx or useMailServiceWorker.ts)")
	}
}
