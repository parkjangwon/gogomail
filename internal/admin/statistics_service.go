package admin

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	UserCount     int64     `json:"user_count"`
	DomainCount   int64     `json:"domain_count"`
	MailboxCount  int64     `json:"mailbox_count"`
	MailCount     int64     `json:"mail_count"`
	StorageUsed   int64     `json:"storage_used"`
	LastUpdated   time.Time `json:"last_updated"`
}

// MetricsCollector interface for collecting system metrics
type MetricsCollector interface {
	GetUserCount(ctx context.Context) (int64, error)
	GetDomainCount(ctx context.Context) (int64, error)
	GetMailboxCount(ctx context.Context) (int64, error)
	GetTotalMailCount(ctx context.Context) (int64, error)
	GetStorageUsed(ctx context.Context) (int64, error)
}

// StatisticsRepository interface for storing metrics
type StatisticsRepository interface {
	GetMetric(ctx context.Context, key string) (interface{}, error)
	SaveMetric(ctx context.Context, key string, value interface{}) error
	GetTimeSeries(ctx context.Context, metric string, startTime, endTime time.Time) ([]interface{}, error)
}

// StatisticsService manages admin dashboard statistics
type StatisticsService struct {
	repo       StatisticsRepository
	collector  MetricsCollector
	cache      map[string]interface{}
	cacheMutex sync.RWMutex
	cacheTime  time.Time
	cacheTTL   time.Duration
}

// NewStatisticsService creates a new statistics service
func NewStatisticsService(repo StatisticsRepository, collector MetricsCollector) *StatisticsService {
	return &StatisticsService{
		repo:      repo,
		collector: collector,
		cache:     make(map[string]interface{}),
		cacheTTL:  5 * time.Minute,
	}
}

// CollectMetrics collects all system metrics
func (ss *StatisticsService) CollectMetrics(ctx context.Context) (*DashboardStats, error) {
	// Check cache first
	if ss.isCacheValid() {
		if stats, ok := ss.cache["dashboard_stats"].(*DashboardStats); ok {
			return stats, nil
		}
	}

	// Collect new metrics
	userCount, err := ss.collector.GetUserCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user count: %w", err)
	}

	domainCount, err := ss.collector.GetDomainCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain count: %w", err)
	}

	mailboxCount, err := ss.collector.GetMailboxCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get mailbox count: %w", err)
	}

	mailCount, err := ss.collector.GetTotalMailCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get mail count: %w", err)
	}

	storageUsed, err := ss.collector.GetStorageUsed(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage used: %w", err)
	}

	stats := &DashboardStats{
		UserCount:    userCount,
		DomainCount:  domainCount,
		MailboxCount: mailboxCount,
		MailCount:    mailCount,
		StorageUsed:  storageUsed,
		LastUpdated:  time.Now(),
	}

	// Save to repository and cache
	ss.repo.SaveMetric(ctx, "user_count", userCount)
	ss.repo.SaveMetric(ctx, "domain_count", domainCount)
	ss.repo.SaveMetric(ctx, "mailbox_count", mailboxCount)
	ss.repo.SaveMetric(ctx, "mail_count", mailCount)
	ss.repo.SaveMetric(ctx, "storage_used", storageUsed)

	ss.setCacheValue("dashboard_stats", stats)

	return stats, nil
}

// GetDashboard returns cached dashboard data
func (ss *StatisticsService) GetDashboard(ctx context.Context) (*DashboardStats, error) {
	ss.cacheMutex.RLock()
	defer ss.cacheMutex.RUnlock()

	if stats, ok := ss.cache["dashboard_stats"].(*DashboardStats); ok {
		return stats, nil
	}

	return nil, fmt.Errorf("dashboard stats not available, call CollectMetrics first")
}

// GetMetric retrieves a specific metric
func (ss *StatisticsService) GetMetric(ctx context.Context, metricKey string) (interface{}, error) {
	if metricKey == "" {
		return nil, fmt.Errorf("%w: metricKey", ErrMissingRequiredField)
	}

	// Try cache first
	if value, ok := ss.getCacheValue(metricKey); ok {
		return value, nil
	}

	// Fallback to repository
	return ss.repo.GetMetric(ctx, metricKey)
}

// RecordTimeSeries records a time-series data point
func (ss *StatisticsService) RecordTimeSeries(ctx context.Context, metric string, value int64) error {
	if metric == "" {
		return fmt.Errorf("%w: metric", ErrMissingRequiredField)
	}

	key := fmt.Sprintf("%s_%d", metric, time.Now().Unix())
	return ss.repo.SaveMetric(ctx, key, value)
}

// InvalidateCache clears the cache
func (ss *StatisticsService) InvalidateCache(ctx context.Context) {
	ss.cacheMutex.Lock()
	defer ss.cacheMutex.Unlock()

	ss.cache = make(map[string]interface{})
	ss.cacheTime = time.Time{}
}

// isCacheValid checks if cache is still valid
func (ss *StatisticsService) isCacheValid() bool {
	ss.cacheMutex.RLock()
	defer ss.cacheMutex.RUnlock()

	if ss.cacheTime.IsZero() {
		return false
	}

	return time.Since(ss.cacheTime) < ss.cacheTTL
}

// setCacheValue sets a value in the cache
func (ss *StatisticsService) setCacheValue(key string, value interface{}) {
	ss.cacheMutex.Lock()
	defer ss.cacheMutex.Unlock()

	ss.cache[key] = value
	ss.cacheTime = time.Now()
}

// getCacheValue gets a value from the cache
func (ss *StatisticsService) getCacheValue(key string) (interface{}, bool) {
	ss.cacheMutex.RLock()
	defer ss.cacheMutex.RUnlock()

	if !ss.isCacheValid() {
		return nil, false
	}

	val, ok := ss.cache[key]
	return val, ok
}
