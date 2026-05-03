package smtpd

import (
	"reflect"
	"testing"

	gosmtp "github.com/emersion/go-smtp"
)

func TestNormalizeDSNRecipientOptionsDeduplicatesNotify(t *testing.T) {
	t.Parallel()

	got := normalizeDSNRecipientOptions("User@Example.COM", &gosmtp.RcptOptions{
		Notify: []gosmtp.DSNNotify{
			gosmtp.DSNNotifyFailure,
			" failure ",
			gosmtp.DSNNotifyDelayed,
		},
	})

	if got.Address != "user@example.com" {
		t.Fatalf("address = %q, want normalized lowercase address", got.Address)
	}
	want := []string{"FAILURE", "DELAY"}
	if !reflect.DeepEqual(got.Notify, want) {
		t.Fatalf("notify = %v, want %v", got.Notify, want)
	}
}
