package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
)

type AuditLogService interface {
	GetByID(ctx context.Context, id string) (*audit.Log, error)
	ListWithFilters(ctx context.Context, filters audit.ListFilters) ([]audit.Log, error)
}

type auditLogResponse struct {
	ID        string            `json:"id"`
	CompanyID string            `json:"company_id,omitempty"`
	DomainID  string            `json:"domain_id,omitempty"`
	UserID    string            `json:"user_id,omitempty"`
	ActorID   string            `json:"actor_id,omitempty"`
	Category  string            `json:"category"`
	Action    string            `json:"action"`
	TargetType string           `json:"target_type"`
	TargetID  string            `json:"target_id,omitempty"`
	IPAddress string            `json:"ip_address,omitempty"`
	UserAgent string            `json:"user_agent"`
	Result    string            `json:"result"`
	Detail    json.RawMessage   `json:"detail"`
	CreatedAt string            `json:"created_at"`
}

type auditLogListResponse struct {
	AuditLogs []auditLogResponse `json:"audit_logs"`
	Limit     int                `json:"limit"`
	Offset    int                `json:"offset"`
	Total     int                `json:"total,omitempty"`
}

func handleAuditLogGet(service AuditLogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing audit log id", http.StatusBadRequest)
			return
		}

		log, err := service.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get audit log: %v", err), http.StatusInternalServerError)
			return
		}
		if log == nil {
			http.Error(w, "audit log not found", http.StatusNotFound)
			return
		}

		resp := auditLogResponse{
			ID:        id,
			CompanyID: log.CompanyID,
			DomainID:  log.DomainID,
			UserID:    log.UserID,
			ActorID:   log.ActorID,
			Category:  log.Category,
			Action:    log.Action,
			TargetType: log.TargetType,
			TargetID:  log.TargetID,
			IPAddress: log.IPAddress,
			UserAgent: log.UserAgent,
			Result:    log.Result,
			Detail:    log.Detail,
			CreatedAt: log.CreatedAt.Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleAuditLogList(service AuditLogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 50
		offset := 0

		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		filters := audit.ListFilters{
			CompanyID:  r.URL.Query().Get("company_id"),
			DomainID:   r.URL.Query().Get("domain_id"),
			UserID:     r.URL.Query().Get("user_id"),
			Category:   r.URL.Query().Get("category"),
			Action:     r.URL.Query().Get("action"),
			TargetType: r.URL.Query().Get("target_type"),
			Limit:      limit,
			Offset:     offset,
		}

		if fromStr := r.URL.Query().Get("from_date"); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				filters.FromDate = &t
			}
		}
		if toStr := r.URL.Query().Get("to_date"); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				filters.ToDate = &t
			}
		}

		logs, err := service.ListWithFilters(r.Context(), filters)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list audit logs: %v", err), http.StatusInternalServerError)
			return
		}

		if logs == nil {
			logs = []audit.Log{}
		}

		items := make([]auditLogResponse, len(logs))
		for i, log := range logs {
			items[i] = auditLogResponse{
				CompanyID: log.CompanyID,
				DomainID:  log.DomainID,
				UserID:    log.UserID,
				ActorID:   log.ActorID,
				Category:  log.Category,
				Action:    log.Action,
				TargetType: log.TargetType,
				TargetID:  log.TargetID,
				IPAddress: log.IPAddress,
				UserAgent: log.UserAgent,
				Result:    log.Result,
				Detail:    log.Detail,
				CreatedAt: log.CreatedAt.Format(time.RFC3339),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(auditLogListResponse{
			AuditLogs: items,
			Limit:     limit,
			Offset:    offset,
		})
	}
}

// RegisterAuditLogRoutes registers audit log endpoints on the admin router with token auth.
func RegisterAuditLogRoutes(mux *http.ServeMux, service AuditLogService, adminToken string) {
	mux.HandleFunc("GET /admin/v1/audit-logs", adminJWTOrStaticAuth(adminToken, nil, handleAuditLogList(service)))
	mux.HandleFunc("GET /admin/v1/audit-logs/{id}", adminJWTOrStaticAuth(adminToken, nil, handleAuditLogGet(service)))
}
