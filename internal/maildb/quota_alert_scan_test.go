package maildb

import (
	"context"
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
