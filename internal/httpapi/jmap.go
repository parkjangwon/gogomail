package httpapi

import (
	"net/http"

	"github.com/gogomail/gogomail/internal/jmap"
)

// RegisterJMAPRoutes registers JMAP endpoints on mux.
//
//   GET  /.well-known/jmap  — Session resource (RFC 8620 §2)
//   POST /jmap/api          — API endpoint (RFC 8620 §3)
func RegisterJMAPRoutes(mux *http.ServeMux, h *jmap.Handler) {
	mux.HandleFunc("GET /.well-known/jmap", h.ServeSession)
	mux.HandleFunc("POST /jmap/api", h.ServeAPI)
}
