package httpapi

import (
	"net/http"
)

func registerAdminDeviceTokenRoutes(mux *http.ServeMux, service AdminService, token string) {
	mux.HandleFunc("GET /admin/v1/users/{id}/device-tokens", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		userID, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		devices, err := service.ListPushDevices(r.Context(), userID, 0)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
	}))

	mux.HandleFunc("DELETE /admin/v1/users/{id}/device-tokens/{device_id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		userID, deviceID, ok := parseBoundedAdminPathPair(w, r, "id", "device_id")
		if !ok {
			return
		}
		if err := service.DeletePushDevice(r.Context(), userID, deviceID); err != nil {
			writeInternalServerError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	}))

	mux.HandleFunc("DELETE /admin/v1/users/{id}/device-tokens", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		userID, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		count, err := service.DeleteAllPushDevices(r.Context(), userID)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": count})
	}))
}
