package orgchart

import (
	"context"
	"errors"
	"testing"
)

func TestNoopAdapterReturnsNotConfigured(t *testing.T) {
	adapter := NoopOrgChartAdapter{}
	got := adapter.SyncOrgChart(context.Background())
	if !errors.Is(got, ErrOrgChartSyncNotConfigured) {
		t.Errorf("NoopOrgChartAdapter.SyncOrgChart() = %v, want ErrOrgChartSyncNotConfigured", got)
	}
}
