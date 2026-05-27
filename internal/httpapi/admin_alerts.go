package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gogomail/gogomail/internal/admin"
)

// handleCreateAlertRule creates a new alert rule.
func handleCreateAlertRule(w http.ResponseWriter, r *http.Request, svc AdminService) {
	companyID, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		AlertType            string  `json:"alert_type"`
		Name                 string  `json:"name"`
		Description          string  `json:"description"`
		Threshold            float64 `json:"threshold"`
		CheckIntervalMinutes int     `json:"check_interval_minutes"`
		IsEnabled            bool    `json:"is_enabled"`
		CreatedBy            string  `json:"created_by"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Name) > 200 {
		writeError(w, http.StatusBadRequest, "name must be 200 characters or fewer")
		return
	}
	if len(req.Description) > 2000 {
		writeError(w, http.StatusBadRequest, "description must be 2000 characters or fewer")
		return
	}
	if len(req.CreatedBy) > 200 {
		writeError(w, http.StatusBadRequest, "created_by must be 200 characters or fewer")
		return
	}

	rule := &admin.AlertRule{
		CompanyID:            companyID,
		AlertType:            req.AlertType,
		Name:                 req.Name,
		Description:          req.Description,
		Threshold:            req.Threshold,
		CheckIntervalMinutes: req.CheckIntervalMinutes,
		IsEnabled:            req.IsEnabled,
		CreatedBy:            req.CreatedBy,
	}

	if err := svc.CreateAlertRule(r.Context(), rule); err != nil {
		slog.ErrorContext(r.Context(), "create alert rule failed", "error", err, "company_id", companyID)
		writeError(w, http.StatusBadRequest, "failed to create alert rule")
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
		Name                 string  `json:"name"`
		Description          string  `json:"description"`
		Threshold            float64 `json:"threshold"`
		CheckIntervalMinutes int     `json:"check_interval_minutes"`
		IsEnabled            bool    `json:"is_enabled"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Name) > 200 {
		writeError(w, http.StatusBadRequest, "name must be 200 characters or fewer")
		return
	}
	if len(req.Description) > 2000 {
		writeError(w, http.StatusBadRequest, "description must be 2000 characters or fewer")
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
		slog.ErrorContext(r.Context(), "update alert rule failed", "error", err, "rule_id", ruleID)
		writeError(w, http.StatusBadRequest, "failed to update alert rule")
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
		ChannelType string                   `json:"channel_type"`
		Name        string                   `json:"name"`
		Config      admin.AlertChannelConfig `json:"config"`
		IsEnabled   bool                     `json:"is_enabled"`
		CreatedBy   string                   `json:"created_by"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
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
		slog.ErrorContext(r.Context(), "create alert channel failed", "error", err, "company_id", companyID)
		writeError(w, http.StatusBadRequest, "failed to create alert channel")
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
		Name      string                   `json:"name"`
		Config    admin.AlertChannelConfig `json:"config"`
		IsEnabled bool                     `json:"is_enabled"`
	}

	if err := decodeJSONBody(r, &req); err != nil {
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
		slog.ErrorContext(r.Context(), "update alert channel failed", "error", err, "channel_id", channelID)
		writeError(w, http.StatusBadRequest, "failed to update alert channel")
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
	if !rejectUnknownQueryKeys(w, r, "limit", "offset", "alert_rule_id", "unresolved") {
		return
	}
	companyID, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	limit, ok := parseAlertEventLimit(w, r)
	if !ok {
		return
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = parsed
	}

	filter := admin.AlertEventFilter{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    offset,
	}
	if alertRuleID := strings.TrimSpace(r.URL.Query().Get("alert_rule_id")); alertRuleID != "" {
		filter.AlertRuleID = alertRuleID
	}
	if unresolved := strings.TrimSpace(r.URL.Query().Get("unresolved")); unresolved != "" {
		switch unresolved {
		case "true", "1":
			filter.OnlyUnresolved = true
		case "false", "0":
			filter.OnlyUnresolved = false
		default:
			writeError(w, http.StatusBadRequest, "invalid unresolved")
			return
		}
	}

	events, hasMore, err := svc.ListAlertEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"events":   events,
		"limit":    limit,
		"offset":   offset,
		"has_more": hasMore,
	})
}

func parseAlertEventLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw, ok := singleQueryValue(w, r, "limit")
	if !ok {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 100, true
	}
	if len(raw) > maxHTTPControlBytes {
		writeError(w, http.StatusBadRequest, "limit is too long")
		return 0, false
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return 0, false
	}
	if limit <= 0 {
		writeError(w, http.StatusBadRequest, "limit must be positive")
		return 0, false
	}
	if limit > 200 {
		writeError(w, http.StatusBadRequest, "limit must be at most 200")
		return 0, false
	}
	return limit, true
}
