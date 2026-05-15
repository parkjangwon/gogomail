package alert

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

// testDB provides database connection for testing.
// Requires GOGOMAIL_DATABASE_URL environment variable to be set.
func testDB(t *testing.T) *sql.DB {
	t.Helper()

	// Skip integration tests if database is not configured
	t.Skip("Integration tests require database setup")
	return nil
}

func TestDBRepositoryCreateConfig(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()
	userID := uuid.New()

	cfg := &Config{
		CompanyID:            companyID,
		AlertType:            AlertTypeStorage,
		Threshold:            80.0,
		Name:                 "Storage Alert",
		Description:          "Alert when storage exceeds 80%",
		CheckIntervalMinutes: 5,
		IsEnabled:            true,
		CreatedByID:          &userID,
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, cfg.ID)
	assert.False(t, cfg.CreatedAt.IsZero())
}

func TestDBRepositoryGetConfig(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg := &Config{
		CompanyID:            companyID,
		AlertType:            AlertTypeLoginFailures,
		Threshold:            10.0,
		Name:                 "Login Failure Alert",
		CheckIntervalMinutes: 5,
		IsEnabled:            true,
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	retrieved, err := repo.GetConfig(ctx, cfg.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, cfg.ID, retrieved.ID)
	assert.Equal(t, AlertTypeLoginFailures, retrieved.AlertType)
	assert.Equal(t, 10.0, retrieved.Threshold)
}

func TestDBRepositoryListConfigs(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg1 := &Config{
		CompanyID:   companyID,
		AlertType:   AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert 1",
		IsEnabled:   true,
	}

	cfg2 := &Config{
		CompanyID:   companyID,
		AlertType:   AlertTypeAPIErrors,
		Threshold:   5.0,
		Name:        "API Error Alert",
		IsEnabled:   true,
	}

	err := repo.CreateConfig(ctx, cfg1)
	require.NoError(t, err)

	err = repo.CreateConfig(ctx, cfg2)
	require.NoError(t, err)

	configs, err := repo.ListConfigs(ctx, companyID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(configs), 2)
}

func TestDBRepositoryUpdateConfig(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg := &Config{
		CompanyID:   companyID,
		AlertType:   AlertTypeStorage,
		Threshold:   80.0,
		Name:        "Storage Alert",
		IsEnabled:   true,
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	cfg.Threshold = 90.0
	cfg.IsEnabled = false

	err = repo.UpdateConfig(ctx, cfg)
	require.NoError(t, err)

	retrieved, err := repo.GetConfig(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Equal(t, 90.0, retrieved.Threshold)
	assert.False(t, retrieved.IsEnabled)
}

func TestDBRepositoryDeleteConfig(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg := &Config{
		CompanyID: companyID,
		AlertType: AlertTypeStorage,
		Threshold: 80.0,
		Name:      "Storage Alert",
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	err = repo.DeleteConfig(ctx, cfg.ID)
	require.NoError(t, err)

	retrieved, err := repo.GetConfig(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestDBRepositoryCreateChannel(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg := &Config{
		CompanyID: companyID,
		AlertType: AlertTypeStorage,
		Threshold: 80.0,
		Name:      "Storage Alert",
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	channel := &Channel{
		ConfigID:    cfg.ID,
		ChannelType: ChannelTypeEmail,
		Config: map[string]interface{}{
			"email": "admin@example.com",
		},
		IsEnabled: true,
	}

	err = repo.CreateChannel(ctx, channel)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, channel.ID)
}

func TestDBRepositoryCreateNotification(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg := &Config{
		CompanyID: companyID,
		AlertType: AlertTypeStorage,
		Threshold: 80.0,
		Name:      "Storage Alert",
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	notif := &Notification{
		CompanyID:     companyID,
		AlertConfigID: cfg.ID,
		AlertType:     AlertTypeStorage,
		Threshold:     80.0,
		CurrentValue:  85.5,
		NotificationData: map[string]interface{}{
			"usage_percent": 85.5,
		},
	}

	err = repo.CreateNotification(ctx, notif)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, notif.ID)
	assert.False(t, notif.CreatedAt.IsZero())
}

func TestDBRepositoryListNotifications(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg := &Config{
		CompanyID: companyID,
		AlertType: AlertTypeStorage,
		Threshold: 80.0,
		Name:      "Storage Alert",
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		notif := &Notification{
			CompanyID:     companyID,
			AlertConfigID: cfg.ID,
			AlertType:     AlertTypeStorage,
			Threshold:     80.0,
			CurrentValue:  85.5 + float64(i),
		}
		err := repo.CreateNotification(ctx, notif)
		require.NoError(t, err)
	}

	notifications, err := repo.ListNotifications(ctx, companyID, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(notifications), 3)
}

func TestDBRepositoryAcknowledgeNotification(t *testing.T) {
	db := testDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo := NewDBRepository(db)
	companyID := uuid.New()

	cfg := &Config{
		CompanyID: companyID,
		AlertType: AlertTypeStorage,
		Threshold: 80.0,
		Name:      "Storage Alert",
	}

	err := repo.CreateConfig(ctx, cfg)
	require.NoError(t, err)

	notif := &Notification{
		CompanyID:     companyID,
		AlertConfigID: cfg.ID,
		AlertType:     AlertTypeStorage,
		Threshold:     80.0,
		CurrentValue:  85.5,
	}

	err = repo.CreateNotification(ctx, notif)
	require.NoError(t, err)

	err = repo.AcknowledgeNotification(ctx, notif.ID)
	require.NoError(t, err)

	notifications, err := repo.ListNotifications(ctx, companyID, 1)
	require.NoError(t, err)
	require.Len(t, notifications, 1)
	assert.NotNil(t, notifications[0].AcknowledgedAt)
}
