package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Status string `json:"status"`
}

type ReadinessResponse struct {
	Status string           `json:"status"`
	Checks []ReadinessCheck `json:"checks"`
}

type ReadinessCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ReadinessCheckFunc func(context.Context) ReadinessCheck

type InfoResponse struct {
	Service                string `json:"service"`
	Status                 string `json:"status"`
	APIVersion             string `json:"api_version"`
	BackendContractVersion string `json:"backend_contract_version"`
}

const (
	APIVersion             = "v1"
	BackendContractVersion = "2026-05-04.backend-release"
)

func RegisterHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health/live", writeHealth)
	mux.HandleFunc("GET /health/ready", writeReadyWithChecks(nil))
	mux.HandleFunc("GET /api/v1/info", writeInfo)
}

func RegisterHealthRoutesWithChecks(mux *http.ServeMux, checks ...ReadinessCheckFunc) {
	mux.HandleFunc("GET /health/live", writeHealth)
	mux.HandleFunc("GET /health/ready", writeReadyWithChecks(checks))
	mux.HandleFunc("GET /api/v1/info", writeInfo)
}

func writeHealth(w http.ResponseWriter, r *http.Request) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

func writeReadyWithChecks(checks []ReadinessCheckFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		response := ReadinessResponse{
			Status: "ok",
			Checks: append([]ReadinessCheck{
				{Name: "http", Status: "ok", Detail: "router registered"},
				{Name: "api_contract", Status: "ok", Detail: "v1 backend-release"},
				{Name: "storage_boundary", Status: "ok", Detail: "configured by runtime mode"},
				{Name: "outbox_boundary", Status: "ok", Detail: "async delivery via outbox"},
			}, runReadinessChecks(r.Context(), checks)...),
		}
		statusCode := http.StatusOK
		for _, check := range response.Checks {
			if check.Status != "ok" {
				response.Status = "degraded"
				statusCode = http.StatusServiceUnavailable
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(response)
	}
}

func runReadinessChecks(ctx context.Context, checks []ReadinessCheckFunc) []ReadinessCheck {
	results := make([]ReadinessCheck, 0, len(checks))
	for _, check := range checks {
		if check == nil {
			continue
		}
		results = append(results, check(ctx))
	}
	return results
}

func StaticReadinessCheck(name string, detail string) ReadinessCheckFunc {
	return func(context.Context) ReadinessCheck {
		return ReadinessCheck{Name: name, Status: "ok", Detail: detail}
	}
}

func writeInfo(w http.ResponseWriter, r *http.Request) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(InfoResponse{
		Service:                "gogomail",
		Status:                 "ok",
		APIVersion:             APIVersion,
		BackendContractVersion: BackendContractVersion,
	})
}
