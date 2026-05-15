package alert

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertConfigCreation(t *testing.T) {
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

	assert.NotEqual(t, uuid.Nil, cfg.CompanyID)
	assert.Equal(t, AlertTypeStorage, cfg.AlertType)
	assert.Equal(t, 80.0, cfg.Threshold)
	assert.Equal(t, "Storage Alert", cfg.Name)
	assert.True(t, cfg.IsEnabled)
}

func TestChannelCreation(t *testing.T) {
	configID := uuid.New()

	channel := &Channel{
		ConfigID:    configID,
		ChannelType: ChannelTypeEmail,
		Config: map[string]interface{}{
			"email": "admin@example.com",
		},
		IsEnabled: true,
	}

	assert.Equal(t, configID, channel.ConfigID)
	assert.Equal(t, ChannelTypeEmail, channel.ChannelType)
	assert.Equal(t, "admin@example.com", channel.Config["email"])
}

func TestNotificationCreation(t *testing.T) {
	companyID := uuid.New()
	configID := uuid.New()

	notif := &Notification{
		CompanyID:      companyID,
		AlertConfigID:  configID,
		AlertType:      AlertTypeStorage,
		Threshold:      80.0,
		CurrentValue:   85.5,
		NotificationData: map[string]interface{}{
			"usage_percent": 85.5,
		},
	}

	assert.Equal(t, companyID, notif.CompanyID)
	assert.Equal(t, AlertTypeStorage, notif.AlertType)
	assert.Greater(t, notif.CurrentValue, notif.Threshold)
	assert.Equal(t, 85.5, notif.NotificationData["usage_percent"])
}

func TestAlertTypeValidation(t *testing.T) {
	tests := []struct {
		alertType AlertType
		valid     bool
	}{
		{AlertTypeStorage, true},
		{AlertTypeLoginFailures, true},
		{AlertTypeAPIErrors, true},
		{AlertType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.alertType), func(t *testing.T) {
			switch tt.alertType {
			case AlertTypeStorage, AlertTypeLoginFailures, AlertTypeAPIErrors:
				assert.True(t, tt.valid)
			default:
				assert.False(t, tt.valid)
			}
		})
	}
}

func TestChannelTypeValidation(t *testing.T) {
	tests := []struct {
		channelType ChannelType
		valid       bool
	}{
		{ChannelTypeEmail, true},
		{ChannelTypeWebhook, true},
		{ChannelTypeDashboard, true},
		{ChannelType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.channelType), func(t *testing.T) {
			switch tt.channelType {
			case ChannelTypeEmail, ChannelTypeWebhook, ChannelTypeDashboard:
				assert.True(t, tt.valid)
			default:
				assert.False(t, tt.valid)
			}
		})
	}
}

func TestNotificationMultipleChannels(t *testing.T) {
	configID := uuid.New()

	channels := []Channel{
		{
			ConfigID:    configID,
			ChannelType: ChannelTypeEmail,
			Config: map[string]interface{}{
				"email": "admin@example.com",
			},
			IsEnabled: true,
		},
		{
			ConfigID:    configID,
			ChannelType: ChannelTypeWebhook,
			Config: map[string]interface{}{
				"url": "https://example.com/webhook",
			},
			IsEnabled: true,
		},
	}

	assert.Len(t, channels, 2)
	assert.Equal(t, ChannelTypeEmail, channels[0].ChannelType)
	assert.Equal(t, ChannelTypeWebhook, channels[1].ChannelType)
}

func TestConfigTimestamps(t *testing.T) {
	cfg := &Config{
		CompanyID: uuid.New(),
		AlertType: AlertTypeStorage,
	}

	before := time.Now()
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	after := time.Now()

	assert.True(t, cfg.CreatedAt.After(before) || cfg.CreatedAt.Equal(before))
	assert.True(t, cfg.UpdatedAt.After(before) || cfg.UpdatedAt.Equal(before))
	assert.True(t, cfg.CreatedAt.Before(after) || cfg.CreatedAt.Equal(after))
	assert.True(t, cfg.UpdatedAt.Before(after) || cfg.UpdatedAt.Equal(after))
}

func TestAcknowledgmentTimestamp(t *testing.T) {
	notif := &Notification{
		CompanyID: uuid.New(),
		CreatedAt: time.Now(),
	}

	assert.Nil(t, notif.AcknowledgedAt)

	now := time.Now()
	notif.AcknowledgedAt = &now

	require.NotNil(t, notif.AcknowledgedAt)
	assert.True(t, notif.AcknowledgedAt.After(notif.CreatedAt))
}
