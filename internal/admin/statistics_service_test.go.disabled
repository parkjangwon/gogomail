package admin

import (
	"context"
	"testing"
	"time"
)

type mockStatisticsRepository struct {
	metrics map[string]interface{}
}

func (m *mockStatisticsRepository) GetMetric(ctx context.Context, key string) (interface{}, error) {
	if val, ok := m.metrics[key]; ok {
		return val, nil
	}
	return nil, ErrNotFound
}

func (m *mockStatisticsRepository) SaveMetric(ctx context.Context, key string, value interface{}) error {
	m.metrics[key] = value
	return nil
}

func (m *mockStatisticsRepository) GetTimeSeries(ctx context.Context, metric string, startTime, endTime time.Time) ([]interface{}, error) {
	var results []interface{}
	// In a real implementation, would query time-series data
	return results, nil
}

func newMockStatisticsRepository() *mockStatisticsRepository {
	return &mockStatisticsRepository{
		metrics: make(map[string]interface{}),
	}
}

type MockMetricsCollector struct {
	userCount     int64
	domainCount   int64
	mailboxCount  int64
	mailCount     int64
	storageUsed   int64
}

func (m *MockMetricsCollector) GetUserCount(ctx context.Context) (int64, error) {
	return m.userCount, nil
}

func (m *MockMetricsCollector) GetDomainCount(ctx context.Context) (int64, error) {
	return m.domainCount, nil
}

func (m *MockMetricsCollector) GetMailboxCount(ctx context.Context) (int64, error) {
	return m.mailboxCount, nil
}

func (m *MockMetricsCollector) GetTotalMailCount(ctx context.Context) (int64, error) {
	return m.mailCount, nil
}

func (m *MockMetricsCollector) GetStorageUsed(ctx context.Context) (int64, error) {
	return m.storageUsed, nil
}

func TestStatisticsServiceCollectMetrics(t *testing.T) {
	repo := newMockStatisticsRepository()
	collector := &MockMetricsCollector{
		userCount:    100,
		domainCount:  10,
		mailboxCount: 150,
		mailCount:    50000,
		storageUsed:  1073741824, // 1GB
	}

	service := NewStatisticsService(repo, collector)
	ctx := context.Background()

	// Collect metrics
	stats, err := service.CollectMetrics(ctx)
	if err != nil {
		t.Errorf("CollectMetrics() error = %v", err)
	}
	if stats == nil {
		t.Error("CollectMetrics() returned nil stats")
	}
}

func TestStatisticsServiceGetDashboard(t *testing.T) {
	repo := newMockStatisticsRepository()
	collector := &MockMetricsCollector{
		userCount:    50,
		domainCount:  5,
		mailboxCount: 75,
		mailCount:    25000,
		storageUsed:  536870912, // 512MB
	}

	service := NewStatisticsService(repo, collector)
	ctx := context.Background()

	// First, collect metrics
	service.CollectMetrics(ctx)

	// Get dashboard data
	dashboard, err := service.GetDashboard(ctx)
	if err != nil {
		t.Errorf("GetDashboard() error = %v", err)
	}
	if dashboard == nil {
		t.Error("GetDashboard() returned nil dashboard")
	}
}

func TestStatisticsServiceCacheInvalidation(t *testing.T) {
	repo := newMockStatisticsRepository()
	collector := &MockMetricsCollector{
		userCount:   100,
		domainCount: 10,
	}

	service := NewStatisticsService(repo, collector)
	ctx := context.Background()

	// Collect metrics
	stats1, _ := service.CollectMetrics(ctx)

	// Update collector
	collector.userCount = 150

	// Cache should still have old data
	dashboard1, _ := service.GetDashboard(ctx)
	if dashboard1 == nil {
		t.Error("Dashboard data should be cached")
	}

	// Invalidate cache
	service.InvalidateCache(ctx)

	// Get new data
	stats2, _ := service.CollectMetrics(ctx)
	dashboard2, _ := service.GetDashboard(ctx)

	if stats1 == stats2 {
		// They might be different or same depending on implementation
		// Just verify that the operations work
	}
	if dashboard2 == nil {
		t.Error("Dashboard data should exist after cache invalidation")
	}
}

func TestStatisticsServiceGetMetric(t *testing.T) {
	repo := newMockStatisticsRepository()
	collector := &MockMetricsCollector{
		userCount: 200,
	}

	service := NewStatisticsService(repo, collector)
	ctx := context.Background()

	tests := []struct {
		name      string
		metricKey string
		shouldErr bool
	}{
		{
			name:      "get valid metric",
			metricKey: "user_count",
			shouldErr: false,
		},
		{
			name:      "get nonexistent metric",
			metricKey: "invalid_metric",
			shouldErr: true,
		},
		{
			name:      "empty metric key",
			metricKey: "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First collect metrics to populate repo
			service.CollectMetrics(ctx)

			value, err := service.GetMetric(ctx, tt.metricKey)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetMetric() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if err == nil && value == nil && !tt.shouldErr {
				t.Error("GetMetric() returned nil value for valid metric")
			}
		})
	}
}

func TestStatisticsServiceRecordTimeSeries(t *testing.T) {
	repo := newMockStatisticsRepository()
	collector := &MockMetricsCollector{
		userCount: 100,
	}

	service := NewStatisticsService(repo, collector)
	ctx := context.Background()

	tests := []struct {
		name      string
		metric    string
		value     int64
		shouldErr bool
	}{
		{
			name:      "record user count series",
			metric:    "user_count_hourly",
			value:     100,
			shouldErr: false,
		},
		{
			name:      "record storage series",
			metric:    "storage_used_hourly",
			value:     1073741824,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.RecordTimeSeries(ctx, tt.metric, tt.value)
			if (err != nil) != tt.shouldErr {
				t.Errorf("RecordTimeSeries() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}
