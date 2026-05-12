package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

// transparentGIF is a 1×1 transparent GIF image.
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
	0x01, 0x00, 0x80, 0x00, 0x00, 0xff, 0xff, 0xff,
	0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

// TrackingRepo is the minimal interface used by tracking handlers.
type TrackingRepo interface {
	RecordTrackingOpen(ctx context.Context, pixelID, ip, userAgent string) error
	ListTrackingEvents(ctx context.Context, senderUserID, messageID string) ([]maildb.TrackingOpenEvent, error)
}

// RegisterTrackingRoutes registers the pixel and tracking query endpoints.
//
//   GET /t/{pixel_id}                    — public, no auth, returns 1×1 GIF
//   GET /api/v1/messages/{id}/tracking   — auth required
func RegisterTrackingRoutes(mux *http.ServeMux, repo TrackingRepo, tokenManager *auth.TokenManager) {
	// Public pixel endpoint — no auth.
	mux.HandleFunc("GET /t/{pixel_id}", func(w http.ResponseWriter, r *http.Request) {
		pixelID := r.PathValue("pixel_id")
		if strings.TrimSpace(pixelID) == "" {
			http.NotFound(w, r)
			return
		}

		ip := clientIP(r)
		ua := r.Header.Get("User-Agent")

		// Fire-and-forget: ignore errors so we always return the GIF.
		_ = repo.RecordTrackingOpen(r.Context(), pixelID, ip, ua)

		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(transparentGIF)
	})

	// Auth-protected tracking query endpoint.
	mux.HandleFunc("GET /api/v1/messages/{id}/tracking", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}

		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		messageID := r.PathValue("id")
		if strings.TrimSpace(messageID) == "" {
			writeError(w, http.StatusBadRequest, "message id is required")
			return
		}

		dbEvents, err := repo.ListTrackingEvents(r.Context(), userID, messageID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load tracking events")
			return
		}

		type eventDTO struct {
			RecipientEmail string  `json:"recipient_email"`
			OpenedAt       *string `json:"opened_at"`
			OpenCount      int     `json:"open_count"`
		}

		dtos := make([]eventDTO, 0, len(dbEvents))
		for _, ev := range dbEvents {
			dto := eventDTO{
				RecipientEmail: ev.RecipientEmail,
				OpenCount:      ev.OpenCount,
			}
			if !ev.OpenedAt.IsZero() {
				s := ev.OpenedAt.UTC().Format(time.RFC3339)
				dto.OpenedAt = &s
			}
			dtos = append(dtos, dto)
		}


		writeJSON(w, http.StatusOK, map[string]any{"events": dtos})
	})
}

// clientIP extracts the real client IP from the request.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if parts := strings.SplitN(fwd, ",", 2); len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
