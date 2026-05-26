package httpapi

import (
	"net/http"

	"github.com/gogomail/gogomail/internal/jmap"
)

// RegisterJMAPRoutes registers JMAP endpoints on mux.
//
//   GET  /.well-known/jmap                          — Session resource (RFC 8620 §2)
//   POST /jmap/api                                  — API endpoint (RFC 8620 §3)
//   POST /jmap/upload/{accountId}/                  — Blob upload (RFC 8620 §6.1)
//   GET  /jmap/download/{accountId}/{blobId}/{name} — Blob download (RFC 8620 §6.2)
//   GET  /jmap/eventsource/                         — EventSource push (RFC 8620 §7.3)
func RegisterJMAPRoutes(mux *http.ServeMux, h *jmap.Handler) {
	mux.HandleFunc("GET /.well-known/jmap", h.ServeSession)
	mux.HandleFunc("POST /jmap/api", h.ServeAPI)
	mux.HandleFunc("POST /jmap/upload/{accountId}/", h.ServeUpload)
	mux.HandleFunc("GET /jmap/download/{accountId}/{blobId}/{name}", h.ServeDownload)
	mux.HandleFunc("GET /jmap/eventsource/", h.ServeEventSource)
}
