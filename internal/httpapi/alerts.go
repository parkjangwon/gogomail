package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gogomail/gogomail/internal/alert"
)

type alertContextKey struct{}

// AlertConfigRequest is the request payload for creating/updating alert configs.
type AlertConfigRequest struct {
	AlertType            string                 `json:"alert_type"`
	Threshold            float64                `json:"threshold"`
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	CheckIntervalMinutes int                    `json:"check_interval_minutes"`
	IsEnabled            bool                   `json:"is_enabled"`
	Channels             []AlertChannelRequest  `json:"channels,omitempty"`
}

// AlertChannelRequest is the request payload for alert notification channels.
type AlertChannelRequest struct {
	ChannelType string                 `json:"channel_type"`
	Config      map[string]interface{} `json:"config"`
	IsEnabled   bool                   `json:"is_enabled"`
}

// AlertConfigResponse is the response payload for alert configs.
type AlertConfigResponse struct {
	ID                   uuid.UUID                 `json:"id"`
	CompanyID            uuid.UUID                 `json:"company_id"`
	AlertType            string                    `json:"alert_type"`
	Threshold            float64                   `json:"threshold"`
	Name                 string                    `json:"name"`
	Description          string                    `json:"description"`
	CheckIntervalMinutes int                       `json:"check_interval_minutes"`
	IsEnabled            bool                      `json:"is_enabled"`
	Channels             []AlertChannelResponse    `json:"channels"`
	CreatedAt            string                    `json:"created_at"`
	UpdatedAt            string                    `json:"updated_at"`
}

// AlertChannelResponse is the response payload for alert channels.
type AlertChannelResponse struct {
	ID          uuid.UUID              `json:"id"`
	ChannelType string                 `json:"channel_type"`
	Config      map[string]interface{} `json:"config"`
	IsEnabled   bool                   `json:"is_enabled"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// AlertNotificationResponse is the response payload for alert notifications.
type AlertNotificationResponse struct {
	ID                uuid.UUID              `json:"id"`
	AlertConfigID     uuid.UUID              `json:"alert_config_id"`
	AlertType         string                 `json:"alert_type"`
	Threshold         float64                `json:"threshold"`
	CurrentValue      float64                `json:"current_value"`
	EmailSent         bool                   `json:"email_sent"`
	WebhookSent       bool                   `json:"webhook_sent"`
	DashboardShown    bool                   `json:"dashboard_shown"`
	NotificationData  map[string]interface{} `json:"notification_data"`
	CreatedAt         string                 `json:"created_at"`
	AcknowledgedAt    *string                `json:"acknowledged_at,omitempty"`
}

// RegisterAlertsRoutes registers alert-related HTTP routes.
func RegisterAlertsRoutes(mux *http.ServeMux, repo alert.Repository) {
	// Config endpoints
	mux.HandleFunc("GET /admin/v1/alerts/configs", makeListAlertConfigsHandler(repo))
	mux.HandleFunc("POST /admin/v1/alerts/configs", makeCreateAlertConfigHandler(repo))
	mux.HandleFunc("GET /admin/v1/alerts/configs/{id}", makeGetAlertConfigHandler(repo))
	mux.HandleFunc("PUT /admin/v1/alerts/configs/{id}", makeUpdateAlertConfigHandler(repo))
	mux.HandleFunc("DELETE /admin/v1/alerts/configs/{id}", makeDeleteAlertConfigHandler(repo))

	// Notification endpoints
	mux.HandleFunc("GET /admin/v1/alerts/notifications", makeListNotificationsHandler(repo))
	mux.HandleFunc("POST /admin/v1/alerts/notifications/{id}/acknowledge", makeAcknowledgeNotificationHandler(repo))
}

// makeListAlertConfigsHandler returns a handler for listing alert configs.
func makeListAlertConfigsHandler(repo alert.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		companyID, ok := ctx.Value(alertContextKey{}).(uuid.UUID)
		if !ok {
			http.Error(w, "company id required", http.StatusBadRequest)
			return
		}

		configs, err := repo.ListConfigs(ctx, companyID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		responses := make([]AlertConfigResponse, 0, len(configs))
		for _, cfg := range configs {
			responses = append(responses, configToResponse(&cfg))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

// makeCreateAlertConfigHandler returns a handler for creating alert configs.
func makeCreateAlertConfigHandler(repo alert.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		companyID, ok := ctx.Value(alertContextKey{}).(uuid.UUID)
		if !ok {
			http.Error(w, "company id required", http.StatusBadRequest)
			return
		}

		var req AlertConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		cfg := &alert.Config{
			CompanyID:            companyID,
			AlertType:            alert.AlertType(req.AlertType),
			Threshold:            req.Threshold,
			Name:                 req.Name,
			Description:          req.Description,
			CheckIntervalMinutes: req.CheckIntervalMinutes,
			IsEnabled:            req.IsEnabled,
		}

		if err := repo.CreateConfig(ctx, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create channels if provided
		for _, chReq := range req.Channels {
			channel := &alert.Channel{
				ConfigID:    cfg.ID,
				ChannelType: alert.ChannelType(chReq.ChannelType),
				Config:      chReq.Config,
				IsEnabled:   chReq.IsEnabled,
			}
			if err := repo.CreateChannel(ctx, channel); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			cfg.Channels = append(cfg.Channels, *channel)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(configToResponse(cfg))
	}
}

// makeGetAlertConfigHandler returns a handler for getting a single alert config.
func makeGetAlertConfigHandler(repo alert.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid config id", http.StatusBadRequest)
			return
		}

		cfg, err := repo.GetConfig(ctx, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if cfg == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configToResponse(cfg))
	}
}

// makeUpdateAlertConfigHandler returns a handler for updating alert configs.
func makeUpdateAlertConfigHandler(repo alert.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid config id", http.StatusBadRequest)
			return
		}

		cfg, err := repo.GetConfig(ctx, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if cfg == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req AlertConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		cfg.AlertType = alert.AlertType(req.AlertType)
		cfg.Threshold = req.Threshold
		cfg.Name = req.Name
		cfg.Description = req.Description
		cfg.CheckIntervalMinutes = req.CheckIntervalMinutes
		cfg.IsEnabled = req.IsEnabled

		if err := repo.UpdateConfig(ctx, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configToResponse(cfg))
	}
}

// makeDeleteAlertConfigHandler returns a handler for deleting alert configs.
func makeDeleteAlertConfigHandler(repo alert.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid config id", http.StatusBadRequest)
			return
		}

		if err := repo.DeleteConfig(ctx, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// makeListNotificationsHandler returns a handler for listing alert notifications.
func makeListNotificationsHandler(repo alert.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		companyID, ok := ctx.Value(alertContextKey{}).(uuid.UUID)
		if !ok {
			http.Error(w, "company id required", http.StatusBadRequest)
			return
		}

		notifications, err := repo.ListNotifications(ctx, companyID, 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		responses := make([]AlertNotificationResponse, 0, len(notifications))
		for _, notif := range notifications {
			responses = append(responses, notificationToResponse(&notif))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

// makeAcknowledgeNotificationHandler returns a handler for acknowledging notifications.
func makeAcknowledgeNotificationHandler(repo alert.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid notification id", http.StatusBadRequest)
			return
		}

		if err := repo.AcknowledgeNotification(ctx, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// Helper functions

func configToResponse(cfg *alert.Config) AlertConfigResponse {
	channels := make([]AlertChannelResponse, 0, len(cfg.Channels))
	for _, ch := range cfg.Channels {
		channels = append(channels, AlertChannelResponse{
			ID:          ch.ID,
			ChannelType: string(ch.ChannelType),
			Config:      ch.Config,
			IsEnabled:   ch.IsEnabled,
			CreatedAt:   ch.CreatedAt.String(),
			UpdatedAt:   ch.UpdatedAt.String(),
		})
	}

	return AlertConfigResponse{
		ID:                   cfg.ID,
		CompanyID:            cfg.CompanyID,
		AlertType:            string(cfg.AlertType),
		Threshold:            cfg.Threshold,
		Name:                 cfg.Name,
		Description:          cfg.Description,
		CheckIntervalMinutes: cfg.CheckIntervalMinutes,
		IsEnabled:            cfg.IsEnabled,
		Channels:             channels,
		CreatedAt:            cfg.CreatedAt.String(),
		UpdatedAt:            cfg.UpdatedAt.String(),
	}
}

func notificationToResponse(notif *alert.Notification) AlertNotificationResponse {
	resp := AlertNotificationResponse{
		ID:               notif.ID,
		AlertConfigID:    notif.AlertConfigID,
		AlertType:        string(notif.AlertType),
		Threshold:        notif.Threshold,
		CurrentValue:     notif.CurrentValue,
		EmailSent:        notif.EmailSent,
		WebhookSent:      notif.WebhookSent,
		DashboardShown:   notif.DashboardShown,
		NotificationData: notif.NotificationData,
		CreatedAt:        notif.CreatedAt.String(),
	}

	if notif.AcknowledgedAt != nil {
		ackStr := notif.AcknowledgedAt.String()
		resp.AcknowledgedAt = &ackStr
	}

	return resp
}
