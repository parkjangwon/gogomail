package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gogomail/gogomail/internal/admin"
)

// handleCreateAlertRule creates a new alert rule.
func handleCreateAlertRule(w http.ResponseWriter, r *http.Request, svc AdminService) {
	companyID, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		AlertType           string `json:"alert_type"`
		Name                string `json:"name"`
		Description         string `json:"description"`
		Threshold           float64 `json:"threshold"`
		CheckIntervalMinutes int    `json:"check_interval_minutes"`
		IsEnabled           bool   `json:"is_enabled"`
		CreatedBy           string `json:"created_by"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule := &admin.AlertRule{
		CompanyID:           companyID,
		AlertType:           req.AlertType,
		Name:                req.Name,
		Description:         req.Description,
		Threshold:           req.Threshold,
		CheckIntervalMinutes: req.CheckIntervalMinutes,
		IsEnabled:           req.IsEnabled,
		CreatedBy:           req.CreatedBy,
	}

	if err := svc.CreateAlertRule(r.Context(), rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

// handleGetAlertRule retrieves an alert rule by ID.
func handleGetAlertRule(w http.ResponseWriter, r *http.Request, svc AdminService) {
	ruleID, ok := parseBoundedAdminPathValue(w, r, "ruleid")
	if !ok {
		return
	}

	rule, err := svc.GetAlertRule(r.Context(), ruleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "alert rule not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

// handleListAlertRules lists all alert rules for a company.
func handleListAlertRules(w http.ResponseWriter, r *http.Request, svc AdminService) {
	companyID, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	rules, err := svc.ListAlertRules(r.Context(), companyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
	})
}

// handleUpdateAlertRule updates an alert rule.
func handleUpdateAlertRule(w http.ResponseWriter, r *http.Request, svc AdminService) {
	ruleID, ok := parseBoundedAdminPathValue(w, r, "ruleid")
	if !ok {
		return
	}

	var req struct {
		Name                string  `json:"name"`
		Description         string  `json:"description"`
		Threshold           float64 `json:"threshold"`
		CheckIntervalMinutes int    `json:"check_interval_minutes"`
		IsEnabled           bool   `json:"is_enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule := &admin.AlertRule{
		ID:                   ruleID,
		Name:                 req.Name,
		Description:          req.Description,
		Threshold:            req.Threshold,
		CheckIntervalMinutes: req.CheckIntervalMinutes,
		IsEnabled:            req.IsEnabled,
	}

	if err := svc.UpdateAlertRule(r.Context(), rule); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "updated",
		"id":     ruleID,
	})
}

// handleDeleteAlertRule deletes an alert rule.
func handleDeleteAlertRule(w http.ResponseWriter, r *http.Request, svc AdminService) {
	ruleID, ok := parseBoundedAdminPathValue(w, r, "ruleid")
	if !ok {
		return
	}

	if err := svc.DeleteAlertRule(r.Context(), ruleID); err != nil {
		writeError(w, http.StatusNotFound, "alert rule not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
	})
}

// handleCreateAlertChannel creates a new alert channel.
func handleCreateAlertChannel(w http.ResponseWriter, r *http.Request, svc AdminService) {
	companyID, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		ChannelType string                     `json:"channel_type"`
		Name        string                     `json:"name"`
		Config      admin.AlertChannelConfig `json:"config"`
		IsEnabled   bool                      `json:"is_enabled"`
		CreatedBy   string                    `json:"created_by"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	channel := &admin.AlertChannel{
		CompanyID:   companyID,
		ChannelType: req.ChannelType,
		Name:        req.Name,
		Config:      req.Config,
		IsEnabled:   req.IsEnabled,
		CreatedBy:   req.CreatedBy,
	}

	if err := svc.CreateAlertChannel(r.Context(), channel); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(channel)
}

// handleListAlertChannels lists all alert channels for a company.
func handleListAlertChannels(w http.ResponseWriter, r *http.Request, svc AdminService) {
	companyID, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	channels, err := svc.ListAlertChannels(r.Context(), companyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list channels")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"channels": channels,
	})
}

// handleUpdateAlertChannel updates an alert channel.
func handleUpdateAlertChannel(w http.ResponseWriter, r *http.Request, svc AdminService) {
	channelID, ok := parseBoundedAdminPathValue(w, r, "channelid")
	if !ok {
		return
	}

	var req struct {
		Name      string                     `json:"name"`
		Config    admin.AlertChannelConfig `json:"config"`
		IsEnabled bool                      `json:"is_enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	channel, err := svc.GetAlertChannel(r.Context(), channelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "alert channel not found")
		return
	}

	channel.Name = req.Name
	channel.IsEnabled = req.IsEnabled
	if len(req.Config.Recipients) > 0 || req.Config.URL != "" || req.Config.AuthHeader != "" {
		channel.Config = req.Config
	}

	if err := svc.UpdateAlertChannel(r.Context(), channel); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channel)
}

// handleDeleteAlertChannel deletes an alert channel.
func handleDeleteAlertChannel(w http.ResponseWriter, r *http.Request, svc AdminService) {
	channelID, ok := parseBoundedAdminPathValue(w, r, "channelid")
	if !ok {
		return
	}

	if err := svc.DeleteAlertChannel(r.Context(), channelID); err != nil {
		writeError(w, http.StatusNotFound, "alert channel not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
	})
}

// handleListAlertEvents lists alert events.
func handleListAlertEvents(w http.ResponseWriter, r *http.Request, svc AdminService) {
	companyID, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	filter := admin.AlertEventFilter{
		CompanyID: companyID,
		Limit:     100,
	}

	events, err := svc.ListAlertEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events": events,
	})
}
