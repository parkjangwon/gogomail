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

	hookText := string(hook)
	pageText := string(page)

	if !strings.Contains(hookText, "export function useMailList(folderId: string, refreshIntervalSeconds: RefreshIntervalSeconds)") {
		t.Fatalf("useMailList must accept refreshIntervalSeconds from Settings")
	}
	if strings.Contains(hookText, "}, 30_000);") {
		t.Fatalf("useMailList must not hard-code a 30s polling interval")
	}
	if !strings.Contains(pageText, "useMailList(activeFolderId, refreshIntervalSeconds)") {
		t.Fatalf("mail page must pass the configured refresh interval into useMailList")
	}
	if !strings.Contains(pageText, "refreshIntervalSeconds * 1000") {
		t.Fatalf("mail page visible-tab refresh interval must be derived from the configured setting")
	}
}
