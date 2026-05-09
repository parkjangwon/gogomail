package orgchart

import (
	"context"
	"testing"
)

func TestNoopAdapterReturnsNil(t *testing.T) {
	adapter := NoopOrgChartAdapter{}
	got := adapter.SyncOrgChart(context.Background())
	if got != nil {
		t.Errorf("NoopOrgChartAdapter.SyncOrgChart() = %v, want nil", got)
	}
}