package httpapi

import (
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Status string `json:"status"`
}

func RegisterHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health/live", writeHealth)
	mux.HandleFunc("GET /health/ready", writeHealth)
}

func writeHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}
