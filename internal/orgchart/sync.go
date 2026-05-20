package orgchart

import (
	"context"
	"errors"
)

var ErrOrgChartSyncNotConfigured = errors.New("organization sync adapter is not configured")

// OrgChartSyncAdapter synchronizes the org chart with an external HR system.
// An external plugin injects an implementation that calls the HR system API.
type OrgChartSyncAdapter interface {
	SyncOrgChart(ctx context.Context) error
}

// NoopOrgChartAdapter marks organization sync as unavailable when no external
// HR system adapter is configured.
type NoopOrgChartAdapter struct{}

func (NoopOrgChartAdapter) SyncOrgChart(context.Context) error {
	return ErrOrgChartSyncNotConfigured
}
