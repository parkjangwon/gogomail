package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestWebmailNativeNotificationsUseCentralStore(t *testing.T) {
	paths := []string{
		"../apps/webmail/src/app/mail/page.tsx",
		"../apps/webmail/src/hooks/useMailList.ts",
		"../apps/webmail/src/components/ComposeModal.tsx",
	}
	for _, path := range paths {
		bodyBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(bodyBytes), "new Notification(") {
			t.Fatalf("%s must route native browser notifications through lib/notifications/store.ts", path)
		}
	}
}
