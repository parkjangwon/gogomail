package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
)

// ─── Webhooks ─────────────────────────────────────────────────────────────────

const webhooksConfigKey = "webhooks_config"

type webhook struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	Secret          string   `json:"secret"`
	SecretSuffix    string   `json:"secret_suffix,omitempty"`
	Events          []string `json:"events"`
	Enabled         bool     `json:"enabled"`
	CreatedAt       string   `json:"created_at"`
	LastTriggeredAt string   `json:"last_triggered_at,omitempty"`
}

type webhooksConfig struct {
	Webhooks []webhook `json:"webhooks"`
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func publicWebhooks(items []webhook) []webhook {
	out := make([]webhook, 0, len(items))
	for _, item := range items {
		if item.SecretSuffix == "" && item.Secret != "" {
			if len(item.Secret) > 8 {
				item.SecretSuffix = item.Secret[len(item.Secret)-8:]
			} else {
				item.SecretSuffix = item.Secret
			}
		}
		item.Secret = ""
		out = append(out, item)
	}
	return out
}

func getWebhooksConfig(ctx context.Context, service AdminService, companyID string) (webhooksConfig, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, webhooksConfigKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return webhooksConfig{Webhooks: []webhook{}}, nil
		}
		return webhooksConfig{}, err
	}
	var cfg webhooksConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return webhooksConfig{Webhooks: []webhook{}}, nil
	}
	if cfg.Webhooks == nil {
		cfg.Webhooks = []webhook{}
	}
	return cfg, nil
}

func saveWebhooksConfig(ctx context.Context, service AdminService, companyID string, cfg webhooksConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = service.SetCompanyConfig(ctx, companyID, webhooksConfigKey, json.RawMessage(b), false, 0)
	return err
}

func handleGetCompanyWebhooks(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": publicWebhooks(cfg.Webhooks)})
}

func handlePostCompanyWebhook(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var input struct {
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		Events  []string `json:"events"`
		Enabled bool     `json:"enabled"`
	}
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if input.Name == "" || input.URL == "" {
		writeError(w, http.StatusBadRequest, "name and url are required")
		return
	}
	governance, err := getCompanySecurityGovernancePolicy(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	allowPrivateNetwork := governance.WebhookPrivateNetworkAccess == "allow"
	parsedURL, err := webhookguard.ValidateOutboundHTTPURL(r.Context(), input.URL, webhookguard.OutboundURLGuardOptions{AllowPrivateNetwork: allowPrivateNetwork})
	if err != nil {
		writeError(w, http.StatusBadRequest, "webhook url is not allowed")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	wh := webhook{
		ID:        fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Name:      input.Name,
		URL:       parsedURL.String(),
		Secret:    randomHex(16),
		Events:    input.Events,
		Enabled:   input.Enabled,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if wh.Events == nil {
		wh.Events = []string{}
	}
	cfg.Webhooks = append(cfg.Webhooks, wh)
	if err := saveWebhooksConfig(r.Context(), service, id, cfg); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"webhook": wh})
}

func handleDeleteCompanyWebhook(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	webhookID := r.PathValue("webhookId")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhookId is required")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	found := false
	filtered := cfg.Webhooks[:0]
	for _, wh := range cfg.Webhooks {
		if wh.ID == webhookID {
			found = true
			continue
		}
		filtered = append(filtered, wh)
	}
	if !found {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	cfg.Webhooks = filtered
	if err := saveWebhooksConfig(r.Context(), service, id, cfg); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleTestCompanyWebhook(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	webhookID := r.PathValue("webhookId")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhookId is required")
		return
	}
	cfg, err := getWebhooksConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var target *webhook
	for i := range cfg.Webhooks {
		if cfg.Webhooks[i].ID == webhookID {
			target = &cfg.Webhooks[i]
			break
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	governance, err := getCompanySecurityGovernancePolicy(r.Context(), service, id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": "security governance policy unavailable"})
		return
	}
	guardOptions := webhookguard.OutboundURLGuardOptions{AllowPrivateNetwork: governance.WebhookPrivateNetworkAccess == "allow"}
	if _, err := webhookguard.ValidateOutboundHTTPURL(r.Context(), target.URL, guardOptions); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": "webhook url is not allowed"})
		return
	}
	payload := fmt.Sprintf(`{"event":"test","timestamp":"%s","data":{"message":"Test webhook from gogomail"}}`,
		time.Now().UTC().Format(time.RFC3339))
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target.URL, strings.NewReader(payload))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": fmt.Sprintf("failed to build request: %v", err)})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gogomail-Event", "test")
	client := webhookguard.GuardedHTTPClient(&http.Client{Timeout: 10 * time.Second}, guardOptions)
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": 0, "message": fmt.Sprintf("request failed: %v", err)})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "status_code": resp.StatusCode, "message": fmt.Sprintf("webhook responded with %d", resp.StatusCode)})
	} else {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "status_code": resp.StatusCode, "message": fmt.Sprintf("webhook responded with %d", resp.StatusCode)})
	}
}

// ─── Notification Templates ───────────────────────────────────────────────────

const notifTemplatesKey = "notification_templates"

type notifTemplate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Enabled bool   `json:"enabled"`
}

type notifTemplatesConfig struct {
	Templates []notifTemplate `json:"templates"`
}

func defaultNotifTemplates() []notifTemplate {
	return []notifTemplate{
		{ID: "password_reset", Name: "Password Reset", Subject: "Reset your {{.CompanyName}} password", Body: "<p>Click the link below to reset your password:</p><p><a href='{{.ResetURL}}'>Reset Password</a></p>", Enabled: true},
		{ID: "welcome", Name: "Welcome Email", Subject: "Welcome to {{.CompanyName}}", Body: "<p>Welcome, {{.UserName}}! Your account has been created.</p>", Enabled: true},
		{ID: "quota_warning", Name: "Quota Warning", Subject: "Storage quota warning — {{.UsagePercent}}% used", Body: "<p>Your mailbox is {{.UsagePercent}}% full. Please free up space or contact your admin.</p>", Enabled: true},
		{ID: "account_locked", Name: "Account Locked", Subject: "Your account has been locked", Body: "<p>Your account has been locked due to too many failed login attempts. Contact your administrator.</p>", Enabled: true},
	}
}

func getNotifTemplatesConfig(ctx context.Context, service AdminService, companyID string) (notifTemplatesConfig, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, notifTemplatesKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return notifTemplatesConfig{Templates: defaultNotifTemplates()}, nil
		}
		return notifTemplatesConfig{}, err
	}
	var cfg notifTemplatesConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return notifTemplatesConfig{Templates: defaultNotifTemplates()}, nil
	}
	if cfg.Templates == nil {
		cfg.Templates = defaultNotifTemplates()
	}
	return cfg, nil
}

func handleGetNotifTemplates(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	cfg, err := getNotifTemplatesConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": cfg.Templates})
}

func handlePutNotifTemplate(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	templateID := r.PathValue("templateId")
	if templateID == "" {
		writeError(w, http.StatusBadRequest, "templateId is required")
		return
	}
	var input notifTemplate
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cfg, err := getNotifTemplatesConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	found := false
	for i := range cfg.Templates {
		if cfg.Templates[i].ID == templateID {
			input.ID = templateID
			cfg.Templates[i] = input
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal templates")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, notifTemplatesKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"template": input})
}

// ─── Audit Log Export ─────────────────────────────────────────────────────────

func handleExportCompanyAuditLogs(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	q := r.URL.Query()
	limit := 1000
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 10000 {
			limit = parsed
		}
	}
	req := maildb.AuditLogListRequest{
		CompanyID:    id,
		Limit:        limit,
		Category:     q.Get("category"),
		ActionPrefix: q.Get("action_prefix"),
	}
	logs, _, err := service.ListAuditLogs(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="audit-logs-%s.csv"`, id))
	wr := csv.NewWriter(w)
	_ = wr.Write([]string{"id", "company_id", "actor_id", "category", "action", "target_type", "target_id", "result", "ip_address", "created_at"})
	for _, l := range logs {
		_ = wr.Write([]string{
			l.ID, l.CompanyID, l.ActorID, l.Category, l.Action,
			l.TargetType, sanitizeCSVCell(l.TargetID), sanitizeCSVCell(l.Result), l.IPAddress,
			l.CreatedAt.Format(time.RFC3339),
		})
	}
	wr.Flush()
}

// ─── Bulk Domain Operations ───────────────────────────────────────────────────

func handleBulkDomains(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	var input struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"` // "activate", "suspend", "delete"
	}
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(input.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if input.Action == "" {
		writeError(w, http.StatusBadRequest, "action is required")
		return
	}
	ctx := r.Context()
	succeeded := []string{}
	failed := []map[string]string{}
	for _, id := range input.IDs {
		var err error
		switch input.Action {
		case "activate":
			err = service.UpdateDomainStatus(ctx, maildb.UpdateDomainStatusRequest{ID: id, Status: "active"})
		case "suspend":
			err = service.UpdateDomainStatus(ctx, maildb.UpdateDomainStatusRequest{ID: id, Status: "suspended"})
		case "delete":
			err = service.DeleteDomain(ctx, id)
		default:
			writeError(w, http.StatusBadRequest, "unknown action: "+input.Action)
			return
		}
		if err != nil {
			failed = append(failed, map[string]string{"id": id, "error": err.Error()})
		} else {
			succeeded = append(succeeded, id)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"succeeded": succeeded,
		"failed":    failed,
	})
}

// ─── Change History ───────────────────────────────────────────────────────────

func handleGetCompanyChangeHistory(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	q := r.URL.Query()
	limit := 100
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	req := maildb.AuditLogListRequest{
		CompanyID:    id,
		Limit:        limit,
		ActionPrefix: q.Get("action_prefix"),
		Category:     q.Get("category"),
		ActorID:      q.Get("actor_id"),
	}
	logs, _, err := service.ListAuditLogs(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": logs, "total": len(logs)})
}

// ─── Pending Approvals ────────────────────────────────────────────────────────

const pendingApprovalsKey = "pending_approvals"

type approvalItem struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Payload     json.RawMessage `json:"payload"`
	RequestedBy string          `json:"requested_by"`
	RequestedAt string          `json:"requested_at"`
	Status      string          `json:"status"`
	ReviewedBy  string          `json:"reviewed_by,omitempty"`
	ReviewedAt  string          `json:"reviewed_at,omitempty"`
	Comment     string          `json:"comment,omitempty"`
}

type approvalsConfig struct {
	Items []approvalItem `json:"items"`
}

func getApprovalsConfig(ctx context.Context, service AdminService, companyID string) (approvalsConfig, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, pendingApprovalsKey)
	if errors.Is(err, configstore.ErrConfigNotFound) {
		return approvalsConfig{Items: []approvalItem{}}, nil
	}
	if err != nil {
		return approvalsConfig{}, err
	}
	var cfg approvalsConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		return approvalsConfig{Items: []approvalItem{}}, nil
	}
	if cfg.Items == nil {
		cfg.Items = []approvalItem{}
	}
	return cfg, nil
}

func saveApprovalsConfig(ctx context.Context, service AdminService, companyID string, cfg approvalsConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = service.SetCompanyConfig(ctx, companyID, pendingApprovalsKey, json.RawMessage(b), false, 0)
	return err
}

func handleGetPendingApprovals(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	out := []approvalItem{}
	for _, item := range cfg.Items {
		if item.Status == status {
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": out})
}

func handleCreatePendingApproval(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var input approvalItem
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if input.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	input.ID = fmt.Sprintf("ap-%d", time.Now().UnixNano())
	input.Status = "pending"
	input.RequestedAt = time.Now().UTC().Format(time.RFC3339)

	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	cfg.Items = append(cfg.Items, input)
	if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"approval": input})
}

func handleApproveApproval(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	approvalID := r.PathValue("approvalId")
	var input struct {
		ReviewedBy string `json:"reviewed_by"`
		Comment    string `json:"comment"`
	}
	_ = decodeJSONBody(r, &input)

	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	for i := range cfg.Items {
		if cfg.Items[i].ID == approvalID {
			cfg.Items[i].Status = "approved"
			cfg.Items[i].ReviewedBy = input.ReviewedBy
			cfg.Items[i].ReviewedAt = time.Now().UTC().Format(time.RFC3339)
			cfg.Items[i].Comment = input.Comment
			if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"approval": cfg.Items[i]})
			return
		}
	}
	writeError(w, http.StatusNotFound, "approval not found")
}

func handleRejectApproval(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	approvalID := r.PathValue("approvalId")
	var input struct {
		ReviewedBy string `json:"reviewed_by"`
		Comment    string `json:"comment"`
	}
	_ = decodeJSONBody(r, &input)

	cfg, err := getApprovalsConfig(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	for i := range cfg.Items {
		if cfg.Items[i].ID == approvalID {
			cfg.Items[i].Status = "rejected"
			cfg.Items[i].ReviewedBy = input.ReviewedBy
			cfg.Items[i].ReviewedAt = time.Now().UTC().Format(time.RFC3339)
			cfg.Items[i].Comment = input.Comment
			if err := saveApprovalsConfig(r.Context(), service, id, cfg); err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"approval": cfg.Items[i]})
			return
		}
	}
	writeError(w, http.StatusNotFound, "approval not found")
}

func handleGetCompanyHealth(w http.ResponseWriter, r *http.Request, service AdminService) {
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	if err := requiresCompanyAccess(ctx, id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	company, err := service.GetCompany(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "company not found")
		return
	}

	domains, _, _ := service.ListDomains(ctx, maildb.DomainListRequest{CompanyID: id, Limit: 200})

	activeDomains := 0
	totalQuotaBytes := int64(0)
	usedQuotaBytes := int64(0)
	overAllocated := false
	for _, d := range domains {
		if d.Status == "active" {
			activeDomains++
		}
		totalQuotaBytes += d.QuotaLimit
		usedQuotaBytes += d.QuotaUsed
		if d.OverAllocated {
			overAllocated = true
		}
	}

	webhooksCfg, _ := getWebhooksConfig(ctx, service, id)
	activeWebhooks := 0
	for _, wh := range webhooksCfg.Webhooks {
		if wh.Enabled {
			activeWebhooks++
		}
	}

	usagePct := 0.0
	if totalQuotaBytes > 0 {
		usagePct = float64(usedQuotaBytes) / float64(totalQuotaBytes) * 100
	}

	healthStatus := "healthy"
	if overAllocated || usagePct > 90 {
		healthStatus = "warning"
	}
	if activeDomains == 0 && len(domains) > 0 {
		healthStatus = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"health": map[string]any{
			"status":          healthStatus,
			"company_id":      id,
			"company_name":    company.Name,
			"domain_count":    len(domains),
			"active_domains":  activeDomains,
			"active_webhooks": activeWebhooks,
			"over_allocated":  overAllocated,
			"quota": map[string]any{
				"total_bytes": totalQuotaBytes,
				"used_bytes":  usedQuotaBytes,
				"usage_pct":   usagePct,
			},
			"checked_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// ─── Route Registration ───────────────────────────────────────────────────────

func registerWebhookRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/webhooks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyWebhooks(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/webhooks", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePostCompanyWebhook(w, r, service)
	}))
	mux.HandleFunc("DELETE /admin/v1/companies/{id}/webhooks/{webhookId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteCompanyWebhook(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/webhooks/{webhookId}/test", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleTestCompanyWebhook(w, r, service)
	}))
}

func registerNotificationTemplateRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/notification-templates", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetNotifTemplates(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/notification-templates/{templateId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutNotifTemplate(w, r, service)
	}))
}

func registerAuditLogExportRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/audit-logs/export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleExportCompanyAuditLogs(w, r, service)
	}))
}

func registerTenantHealthRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/health", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyHealth(w, r, service)
	}))
}

func registerChangeHistoryAndApprovalsRoutes(mux *http.ServeMux, adminAuth func(http.HandlerFunc) http.HandlerFunc, service AdminService) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/change-history", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyChangeHistory(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/pending-approvals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetPendingApprovals(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleCreatePendingApproval(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals/{approvalId}/approve", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleApproveApproval(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/pending-approvals/{approvalId}/reject", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleRejectApproval(w, r, service)
	}))
}
