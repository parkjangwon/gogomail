package orgchart

import (
	"context"
)

// OrgChartSyncAdapter synchronizes the org chart with an external HR system.
// An external plugin injects an implementation that calls the HR system API.
// When no adapter is configured, the no-op implementation is used.
type OrgChartSyncAdapter interface {
	SyncOrgChart(ctx context.Context) error
}

// NoopOrgChartAdapter is the default adapter used when no external HR system
// adapter is configured. It does nothing and always returns nil.
type NoopOrgChartAdapter struct{}

func (NoopOrgChartAdapter) SyncOrgChart(context.Context) error {
	return nil
}
