package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/gogomail/gogomail/internal/alert"
)

// mockAlertRepository is a test implementation of alert.Repository.
type mockAlertRepository struct {
	configs       []alert.Config
	notifications []alert.Notification
}

func newMockAlertRepository() *mockAlertRepository {
	return &mockAlertRepository{
		configs:       []alert.Config{},
		notifications: []alert.Notification{},
	}
}

func (m *mockAlertRepository) CreateConfig(ctx context.Context, cfg *alert.Config) error {
	cfg.ID = uuid.New()
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	m.configs = append(m.configs, *cfg)
	return nil
}

func (m *mockAlertRepository) GetConfig(ctx context.Context, id uuid.UUID) (*alert.Config, error) {
	for i, cfg := range m.configs {
		if cfg.ID == id {
			return &m.configs[i], nil
		}
	}
	return nil, nil
}

func (m *mockAlertRepository) ListConfigs(ctx context.Context, companyID uuid.UUID) ([]alert.Config, error) {
	var result []alert.Config
	for _, cfg := range m.configs {
		if cfg.CompanyID == companyID {
			result = append(result, cfg)
		}
	}
	return result, nil
}

func (m *mockAlertRepository) UpdateConfig(ctx context.Context, cfg *alert.Config) error {
	for i, c := range m.configs {
		if c.ID == cfg.ID {
			m.configs[i] = *cfg
			return nil
		}
	}
	return nil
}

func (m *mockAlertRepository) DeleteConfig(ctx context.Context, id uuid.UUID) error {
	for i, cfg := range m.configs {
		if cfg.ID == id {
			m.configs = append(m.configs[:i], m.configs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockAlertRepository) CreateChannel(ctx context.Context, channel *alert.Channel) error {
	channel.ID = uuid.New()
	channel.CreatedAt = time.Now()
	channel.UpdatedAt = time.Now()
	return nil
}

func (m *mockAlertRepository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockAlertRepository) CreateNotification(ctx context.Context, notif *alert.Notification) error {
	notif.ID = uuid.New()
	notif.CreatedAt = time.Now()
	m.notifications = append(m.notifications, *notif)
	return nil
}

func (m *mockAlertRepository) ListNotifications(ctx context.Context, companyID uuid.UUID, limit int) ([]alert.Notification, error) {
	var result []alert.Notification
	for _, notif := range m.notifications {
		if notif.CompanyID == companyID {
			result = append(result, notif)
		}
	}
	return result, nil
}

func (m *mockAlertRepository) AcknowledgeNotification(ctx context.Context, id uuid.UUID) error {
	for i, notif := range m.notifications {
		if notif.ID == id {
			now := time.Now()
			m.notifications[i].AcknowledgedAt = &now
			return nil
		}
	}
	return nil
}

func TestListAlertConfigs(t *testing.T) {
	repo := newMockAlertRepository()
	companyID := uuid.New()

	cfg := &alert.Config{
		CompanyID:   companyID,
		AlertType:   alert.AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	repo.CreateConfig(context.Background(), cfg)

	router := http.NewServeMux()
	RegisterAlertsRoutes(router, repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/alerts/configs", nil)
	req = req.WithContext(context.WithValue(req.Context(), alertContextKey{}, companyID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var configs []AlertConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &configs)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(configs), 1)
}

func TestCreateAlertConfig(t *testing.T) {
	repo := newMockAlertRepository()
	companyID := uuid.New()

	req := &AlertConfigRequest{
		AlertType:            "storage",
		Threshold:            80.0,
		Name:                 "Storage Alert",
		Description:          "Alert when storage exceeds 80%",
		CheckIntervalMinutes: 5,
		IsEnabled:            true,
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	router := http.NewServeMux()
	RegisterAlertsRoutes(router, repo)

	httpReq := httptest.NewRequest(http.MethodPost, "/admin/v1/alerts/configs", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), alertContextKey{}, companyID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusCreated, w.Code)

	var cfg AlertConfigResponse
	err = json.Unmarshal(w.Body.Bytes(), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "storage", cfg.AlertType)
	assert.Equal(t, 80.0, cfg.Threshold)
}

func TestGetAlertConfig(t *testing.T) {
	repo := newMockAlertRepository()
	companyID := uuid.New()

	cfg := &alert.Config{
		CompanyID:   companyID,
		AlertType:   alert.AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	repo.CreateConfig(context.Background(), cfg)

	router := http.NewServeMux()
	RegisterAlertsRoutes(router, repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/alerts/configs/"+cfg.ID.String(), nil)
	req = req.WithContext(context.WithValue(req.Context(), alertContextKey{}, companyID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AlertConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, cfg.ID, response.ID)
}

func TestUpdateAlertConfig(t *testing.T) {
	repo := newMockAlertRepository()
	companyID := uuid.New()

	cfg := &alert.Config{
		CompanyID:   companyID,
		AlertType:   alert.AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	repo.CreateConfig(context.Background(), cfg)

	updateReq := &AlertConfigRequest{
		AlertType:            "storage",
		Threshold:            90.0,
		Name:                 "Updated Storage Alert",
		CheckIntervalMinutes: 5,
		IsEnabled:            true,
	}

	body, err := json.Marshal(updateReq)
	require.NoError(t, err)

	router := http.NewServeMux()
	RegisterAlertsRoutes(router, repo)

	req := httptest.NewRequest(http.MethodPut, "/admin/v1/alerts/configs/"+cfg.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), alertContextKey{}, companyID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AlertConfigResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 90.0, response.Threshold)
	assert.Equal(t, "Updated Storage Alert", response.Name)
}

func TestDeleteAlertConfig(t *testing.T) {
	repo := newMockAlertRepository()
	companyID := uuid.New()

	cfg := &alert.Config{
		CompanyID:   companyID,
		AlertType:   alert.AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	repo.CreateConfig(context.Background(), cfg)

	router := http.NewServeMux()
	RegisterAlertsRoutes(router, repo)

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/alerts/configs/"+cfg.ID.String(), nil)
	req = req.WithContext(context.WithValue(req.Context(), alertContextKey{}, companyID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestListNotifications(t *testing.T) {
	repo := newMockAlertRepository()
	companyID := uuid.New()

	cfg := &alert.Config{
		CompanyID:   companyID,
		AlertType:   alert.AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	repo.CreateConfig(context.Background(), cfg)

	notif := &alert.Notification{
		CompanyID:     companyID,
		AlertConfigID: cfg.ID,
		AlertType:     alert.AlertTypeStorage,
		Threshold:     80.0,
		CurrentValue:  85.0,
	}
	repo.CreateNotification(context.Background(), notif)

	router := http.NewServeMux()
	RegisterAlertsRoutes(router, repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/alerts/notifications", nil)
	req = req.WithContext(context.WithValue(req.Context(), alertContextKey{}, companyID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var notifications []AlertNotificationResponse
	err := json.Unmarshal(w.Body.Bytes(), &notifications)
	require.NoError(t, err)
	assert.Len(t, notifications, 1)
}

func TestAcknowledgeNotification(t *testing.T) {
	repo := newMockAlertRepository()
	companyID := uuid.New()

	cfg := &alert.Config{
		CompanyID:   companyID,
		AlertType:   alert.AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}
	repo.CreateConfig(context.Background(), cfg)

	notif := &alert.Notification{
		CompanyID:     companyID,
		AlertConfigID: cfg.ID,
		AlertType:     alert.AlertTypeStorage,
		Threshold:     80.0,
		CurrentValue:  85.0,
	}
	repo.CreateNotification(context.Background(), notif)

	router := http.NewServeMux()
	RegisterAlertsRoutes(router, repo)

	req := httptest.NewRequest(http.MethodPost, "/admin/v1/alerts/notifications/"+notif.ID.String()+"/acknowledge", nil)
	req = req.WithContext(context.WithValue(req.Context(), alertContextKey{}, companyID))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
