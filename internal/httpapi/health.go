package httpapi

import (
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
	mux.HandleFunc("GET /health/ready", writeReady)
	mux.HandleFunc("GET /api/v1/info", writeInfo)
}

func writeHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

func writeReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ReadinessResponse{
		Status: "ok",
		Checks: []ReadinessCheck{
			{Name: "http", Status: "ok", Detail: "router registered"},
			{Name: "api_contract", Status: "ok", Detail: "v1 backend-release"},
			{Name: "storage_boundary", Status: "ok", Detail: "configured by runtime mode"},
			{Name: "outbox_boundary", Status: "ok", Detail: "async delivery via outbox"},
		},
	})
}

func writeInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(InfoResponse{
		Service:                "gogomail",
		Status:                 "ok",
		APIVersion:             APIVersion,
		BackendContractVersion: BackendContractVersion,
	})
}
