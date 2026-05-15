package alert

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a test implementation of Repository.
type MockRepository struct {
	configs       []Config
	channels      []Channel
	notifications []Notification
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		configs:       []Config{},
		channels:      []Channel{},
		notifications: []Notification{},
	}
}

func (m *MockRepository) CreateConfig(ctx context.Context, cfg *Config) error {
	cfg.ID = uuid.New()
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	m.configs = append(m.configs, *cfg)
	return nil
}

func (m *MockRepository) GetConfig(ctx context.Context, id uuid.UUID) (*Config, error) {
	for _, cfg := range m.configs {
		if cfg.ID == id {
			return &cfg, nil
		}
	}
	return nil, nil
}

func (m *MockRepository) ListConfigs(ctx context.Context, companyID uuid.UUID) ([]Config, error) {
	var result []Config
	for _, cfg := range m.configs {
		if cfg.CompanyID == companyID {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (m *MockRepository) UpdateConfig(ctx context.Context, cfg *Config) error {
	for i, c := range m.configs {
		if c.ID == cfg.ID {
			m.configs[i] = *cfg
			return nil
		}
	}
	return nil
}

func (m *MockRepository) DeleteConfig(ctx context.Context, id uuid.UUID) error {
	for i, cfg := range m.configs {
		if cfg.ID == id {
			m.configs = append(m.configs[:i], m.configs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MockRepository) CreateChannel(ctx context.Context, channel *Channel) error {
	channel.ID = uuid.New()
	channel.CreatedAt = time.Now()
	channel.UpdatedAt = time.Now()
	m.channels = append(m.channels, *channel)
	return nil
}

func (m *MockRepository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	for i, ch := range m.channels {
		if ch.ID == id {
			m.channels = append(m.channels[:i], m.channels[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MockRepository) CreateNotification(ctx context.Context, notif *Notification) error {
	notif.ID = uuid.New()
	notif.CreatedAt = time.Now()
	m.notifications = append(m.notifications, *notif)
	return nil
}

func (m *MockRepository) ListNotifications(ctx context.Context, companyID uuid.UUID, limit int) ([]Notification, error) {
	var result []Notification
	for _, notif := range m.notifications {
		if notif.CompanyID == companyID {
			result = append(result, notif)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockRepository) AcknowledgeNotification(ctx context.Context, id uuid.UUID) error {
	for i, notif := range m.notifications {
		if notif.ID == id {
			now := time.Now()
			m.notifications[i].AcknowledgedAt = &now
			return nil
		}
	}
	return nil
}

func TestEvaluateStorageAlert(t *testing.T) {
	ctx := context.Background()
	companyID := uuid.New()
	mockRepo := NewMockRepository()
	evaluator := NewDefaultEvaluator(mockRepo)

	// Create storage alert config
	cfg := &Config{
		CompanyID:            companyID,
		AlertType:            AlertTypeStorage,
		Threshold:            80.0,
		Name:                 "Storage Alert",
		IsEnabled:            true,
		CheckIntervalMinutes: 5,
	}
	err := mockRepo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	// Evaluate storage below threshold - no alert
	err = evaluator.EvaluateStorage(ctx, companyID, 75.0)
	require.NoError(t, err)
	assert.Len(t, mockRepo.notifications, 0)

	// Evaluate storage above threshold - alert triggered
	err = evaluator.EvaluateStorage(ctx, companyID, 85.0)
	require.NoError(t, err)
	assert.Len(t, mockRepo.notifications, 1)

	notif := mockRepo.notifications[0]
	assert.Equal(t, AlertTypeStorage, notif.AlertType)
	assert.Equal(t, 80.0, notif.Threshold)
	assert.Equal(t, 85.0, notif.CurrentValue)
}

func TestEvaluateLoginFailuresAlert(t *testing.T) {
	ctx := context.Background()
	companyID := uuid.New()
	mockRepo := NewMockRepository()
	evaluator := NewDefaultEvaluator(mockRepo)

	// Create login failure alert config
	cfg := &Config{
		CompanyID:            companyID,
		AlertType:            AlertTypeLoginFailures,
		Threshold:            10.0,
		Name:                 "Login Failure Alert",
		IsEnabled:            true,
		CheckIntervalMinutes: 5,
	}
	err := mockRepo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	// Evaluate failures below threshold - no alert
	err = evaluator.EvaluateLoginFailures(ctx, companyID, 5)
	require.NoError(t, err)
	assert.Len(t, mockRepo.notifications, 0)

	// Evaluate failures above threshold - alert triggered
	err = evaluator.EvaluateLoginFailures(ctx, companyID, 15)
	require.NoError(t, err)
	assert.Len(t, mockRepo.notifications, 1)

	notif := mockRepo.notifications[0]
	assert.Equal(t, AlertTypeLoginFailures, notif.AlertType)
	assert.Equal(t, 10.0, notif.Threshold)
	assert.Equal(t, 15.0, notif.CurrentValue)
}

func TestEvaluateAPIErrorsAlert(t *testing.T) {
	ctx := context.Background()
	companyID := uuid.New()
	mockRepo := NewMockRepository()
	evaluator := NewDefaultEvaluator(mockRepo)

	// Create API error rate alert config
	cfg := &Config{
		CompanyID:            companyID,
		AlertType:            AlertTypeAPIErrors,
		Threshold:            5.0,
		Name:                 "API Error Alert",
		IsEnabled:            true,
		CheckIntervalMinutes: 5,
	}
	err := mockRepo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	// Evaluate error rate below threshold - no alert
	err = evaluator.EvaluateAPIErrors(ctx, companyID, 2.5)
	require.NoError(t, err)
	assert.Len(t, mockRepo.notifications, 0)

	// Evaluate error rate above threshold - alert triggered
	err = evaluator.EvaluateAPIErrors(ctx, companyID, 7.5)
	require.NoError(t, err)
	assert.Len(t, mockRepo.notifications, 1)

	notif := mockRepo.notifications[0]
	assert.Equal(t, AlertTypeAPIErrors, notif.AlertType)
	assert.Equal(t, 5.0, notif.Threshold)
	assert.Equal(t, 7.5, notif.CurrentValue)
}

func TestEvaluateDisabledAlert(t *testing.T) {
	ctx := context.Background()
	companyID := uuid.New()
	mockRepo := NewMockRepository()
	evaluator := NewDefaultEvaluator(mockRepo)

	// Create disabled storage alert config
	cfg := &Config{
		CompanyID:            companyID,
		AlertType:            AlertTypeStorage,
		Threshold:            80.0,
		Name:                 "Storage Alert",
		IsEnabled:            false,
		CheckIntervalMinutes: 5,
	}
	err := mockRepo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	// Evaluate storage above threshold - no alert because config is disabled
	err = evaluator.EvaluateStorage(ctx, companyID, 85.0)
	require.NoError(t, err)
	assert.Len(t, mockRepo.notifications, 0)
}

func TestEvaluateMultipleAlerts(t *testing.T) {
	ctx := context.Background()
	companyID := uuid.New()
	mockRepo := NewMockRepository()
	evaluator := NewDefaultEvaluator(mockRepo)

	// Create multiple alert configs
	storageAlert := &Config{
		CompanyID:   companyID,
		AlertType:   AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	loginAlert := &Config{
		CompanyID:   companyID,
		AlertType:   AlertTypeLoginFailures,
		Threshold:   10.0,
		Name:        "Login Alert",
		IsEnabled:   true,
	}

	mockRepo.CreateConfig(ctx, storageAlert)
	mockRepo.CreateConfig(ctx, loginAlert)

	// Trigger both alerts
	evaluator.EvaluateStorage(ctx, companyID, 85.0)
	evaluator.EvaluateLoginFailures(ctx, companyID, 15)

	assert.Len(t, mockRepo.notifications, 2)
}

func TestNotificationData(t *testing.T) {
	ctx := context.Background()
	companyID := uuid.New()
	mockRepo := NewMockRepository()
	evaluator := NewDefaultEvaluator(mockRepo)

	cfg := &Config{
		CompanyID:   companyID,
		AlertType:   AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	mockRepo.CreateConfig(ctx, cfg)

	evaluator.EvaluateStorage(ctx, companyID, 85.0)

	notif := mockRepo.notifications[0]
	assert.NotNil(t, notif.NotificationData)
	assert.Equal(t, 85.0, notif.NotificationData["usage_percent"])
	assert.Equal(t, 80.0, notif.NotificationData["threshold"])
	assert.NotZero(t, notif.NotificationData["at"])
}
