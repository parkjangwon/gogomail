package maildb

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestScanAndRecordQuotaAlertsRejectsNilDB(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.ScanAndRecordQuotaAlerts(context.Background(), 0.80, 0.95)
	if err == nil {
		t.Fatal("ScanAndRecordQuotaAlerts with nil db should return error")
	}
}

func TestScanAndRecordQuotaAlertsRejectsInvalidWarningRatio(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.ScanAndRecordQuotaAlerts(context.Background(), 0.0, 0.95)
	if err == nil {
		t.Fatal("ScanAndRecordQuotaAlerts with zero warning ratio should return error")
	}
}

func TestScanAndRecordQuotaAlertsRejectsInvalidCriticalRatio(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.ScanAndRecordQuotaAlerts(context.Background(), 0.80, 1.1)
	if err == nil {
		t.Fatal("ScanAndRecordQuotaAlerts with critical ratio > 1 should return error")
	}
}

func TestScanAndRecordQuotaAlertsRejectsInvertedRatios(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.ScanAndRecordQuotaAlerts(context.Background(), 0.95, 0.80)
	if err == nil {
		t.Fatal("ScanAndRecordQuotaAlerts with warning > critical should return error")
	}
}

func TestScanAndRecordQuotaAlertsKeepsDuplicateCheckSargable(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("quota_alert_scan.go")
	if err != nil {
		t.Fatalf("read quota_alert_scan.go: %v", err)
	}
	for _, want := range []string{
		"qa.company_id = c.company_id::uuid",
		"qa.user_id = c.entity_id::uuid",
		"qa.domain_id = c.entity_id::uuid",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("quota alert scan query missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"qa.company_id::text =",
		"qa.user_id::text",
		"qa.domain_id::text",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("quota alert scan duplicate check casts indexed column: %s", forbidden)
		}
	}
}
